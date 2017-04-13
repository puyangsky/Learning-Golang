package lsp

import (
	"github.com/lspnet"
	"encoding/json"
	"errors"
	"time"
)

type client struct {
	hostport 	string
	params   	*Params
	udpConn  	*lspnet.UDPConn
	connID   	int 			// 客户端的连接ID
	readChan 	chan *Message
	writeChan	chan *Message
	closeChan	chan byte
	alive		bool			//服务器是否在线
	seqNum		int 			// 消息序列号，只有发送data message时才增长
	timer 		*timer
}

// NewClient creates, initiates, and returns a new client. This function
// should return after a connection with the server has been established
// (i.e., the client has received an Ack message from the server in response
// to its connection request), and should return a non-nil error if a
// connection could not be made (i.e., if after K epochs, the client still
// hasn't received an Ack message from the server in response to its K
// connection requests).
//
// hostport is a colon-separated string identifying the server's host address
// and port number (i.e., "localhost:9999").

func NewClient(hostport string, params *Params) (Client, error) {

	udpAddr, err := lspnet.ResolveUDPAddr("udp4", hostport)
	HandleErr(err)

	udpConn, err := lspnet.DialUDP("udp4", nil, udpAddr)
	HandleErr(err)

	defer udpConn.Close()

	var ackMsg *Message
	connectMsg := NewConnect()
	bytes, _ := json.Marshal(connectMsg)
	//println(connectMsg.String())
	//尝试连接服务器
	for i := 0; i < params.EpochLimit && ackMsg == nil; i++ {
		//println(i+1, "th trial to connect server")
		timeoutChan := time.After(time.Millisecond * time.Duration(params.EpochMillis))
		select {
		case <-timeoutChan:
			println(i, "th timeout")
			continue
		default:
			n, err := udpConn.Write(bytes)
			if err != nil {
				println(err.Error())
				continue
			}
			//println("write to server, send ", n)

			var ack = make([]byte, 2000)
			n, err = udpConn.Read(ack[0:])
			if err != nil {
				println(err.Error())
				continue
			}
			//println("read from server, get ", n)
			err = json.Unmarshal(ack[:n], &ackMsg)
			if err != nil {
				println(err.Error())
				continue
			}
		}
	}

	if ackMsg == nil {
		return nil, errors.New("连接超时")
	}

	timer := &timer{
		timerChan:		make(chan *Message),
		timeoutChanMap: make(map[int] chan *Message),
		epochTimes:		make(map[int]int),
	}
	client := &client{
		hostport: 	hostport,
		params:   	params,
		udpConn:  	udpConn,
		connID:   	ackMsg.ConnID,
		readChan: 	make(chan *Message),
		writeChan:	make(chan *Message, params.WindowSize),
		closeChan:	make(chan byte),
		alive:	  	true,
		seqNum:		1,
		timer:		timer,
	}

	go client.handleRead()
	go client.handleWrite()
	go client.handleTimeout()

	return client, nil
}

func (c *client) ConnID() int {
	return c.connID
}

func (c *client) Read() ([]byte, error) {
	select {
	case <- c.closeChan:
		return nil, errors.New("[Error] Connection closed!")
	//真正读消息的地方
	case msg := <- c.readChan:
		println("[INFO] Receive message from client: ", msg.String())
		//读完消息后要返回一个ACK消息给服务器端
		ackMsg := NewAck(c.connID, msg.SeqNum)
		c.writeChan <- ackMsg
		return msg.Payload, nil
	}
}

//发送消息
func (c *client) Write(payload []byte) error {
	if c.alive == false {
		return errors.New("[Error] Connection closed!")
	}
	dataMsg := NewData(c.connID, c.seqNum, payload)
	c.seqNum++
	//塞到channel中，此处为none-block
	c.writeChan <- dataMsg
	return nil
}

func (c *client) Close() error {
	close(c.closeChan)
	c.alive = false
	err := c.udpConn.Close()
	return err
}

//用goroutine来不断监听读事件
func (c *client) handleRead() {
	for{
		payload := make([]byte, 1000)
		n, err := c.udpConn.Read(payload)
		if err != nil {
			return
		}
		//反序列化消息
		var message *Message
		err = json.Unmarshal(payload[:n], &message)

		switch message.Type {
		case MsgData:
			// 过滤已经读过的消息
			if message.SeqNum < c.seqNum {
				//把从服务器端读取到的消息塞到readChan中
				c.readChan <- message
				go c.sendAck(message)
			}
		case MsgAck:
			//接收到ACK，解除超时警报
			var timeoutChan = make(chan *Message)
			timeoutChan <- message
			c.timer.timeoutChanMap[message.SeqNum] = timeoutChan
		}

	}
}

func (c *client) handleWrite() {
	for {
		select {
		case msg := <- c.writeChan:
			//println("[INFO] Send message to server: ", msg.String())
			if msg.Type == MsgConnect {
				continue
			}
			if c.alive {
				println("[INFO] Client send message to server: ", msg.String())
				//序列化消息
				bytes, err := json.Marshal(msg)
				if err != nil {
					continue
				}
				//真正写消息的地方
				_, err = c.udpConn.Write(bytes)
				if err != nil {
					continue
				}
				//写完消息，触发计时器，如果超时没收到ACK要重新发送消息
				if msg.Type == MsgConnect || msg.Type == MsgData {
					c.timer.timerChan <- msg
				}
			}
		case <-c.closeChan:
			return
		}
	}
}

/*
发送ack
 */
func (c *client) sendAck(msg *Message)  {
	ack := NewAck(msg.ConnID, msg.SeqNum)
	c.writeChan <- ack
}

/*
客户端处理超时的协程
 */
func (c *client) handleTimeout() {
	for{
		select {
		//开始计时
		case timerMsg := <- c.timer.timerChan:
			timeoutChan := time.After(time.Millisecond * time.Duration(c.params.EpochMillis))
			select {
				//连接超时
				case <- timeoutChan:
					times, ok := c.timer.epochTimes[timerMsg.SeqNum]
					if ok && times < c.params.EpochLimit {
						go c.resend(timerMsg)
					}else {
						//达到最大次数，判定连接断开
						println("[DEBUG] Message timeout, exiting...")
						c.alive = false
						close(c.closeChan)
					}

				//收到dataMsg的ack
				case <- c.timer.timeoutChanMap[timerMsg.SeqNum]:
					continue
				}
		case <- c.closeChan:
			return
		}
	}
}

/*
客户器端重发消息
 */
func (c *client) resend(msg *Message)  {
	payload, err := json.Marshal(msg)
	if err != nil {
		println(err.Error())
		return
	}
	_, err = c.udpConn.Write(payload)
	if err != nil {
		println(err.Error())
		return
	}
	//再次加入计时器
	c.timer.timerChan <- msg
	//epoch次数加一
	c.timer.epochTimes[msg.SeqNum]++
}