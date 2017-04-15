package lsp

import (
	"github.com/lspnet"
	"encoding/json"
	"errors"
	"time"
	"sync/atomic"
)

type client struct {
	hostport 	string
	params   	*Params
	udpConn  	*lspnet.UDPConn
	connID   	int 			// 客户端的连接ID
	readChan 	chan *Message
	writeChan	chan *Message
	closeChan	chan byte
	alive		bool			// 服务器是否在线
	seqNum		int32 			// 消息序列号，只有发送data message时才增长
	timers 		map[int]*timer	// 计时器数组
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

	//defer udpConn.Close()

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

	//timer := &timer{
	//	timerChan:		make(chan *Message),
	//	timeoutChanMap: make(chan bool),
	//	epochTimes:		0,
	//}
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
		timers:		make(map[int] *timer),
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
		return nil, errors.New("<<<[Error] Connection closed!")
	//真正读消息的地方
	case msg := <- c.readChan:
		println("<<<[INFO] Client receive message from server: ", msg.String())
		//读完消息后要返回一个ACK消息给服务器端
		go c.sendAck(msg)
		return msg.Payload, nil
	}
}

//发送消息
func (c *client) Write(payload []byte) error {
	if c.alive == false {
		return errors.New("<<<[Error] Connection closed!")
	}
	dataMsg := NewData(c.connID, int(c.seqNum), payload)
	//创建新的dataMsg时，自增seqNum
	c.atomicAddSeq()
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
			println(err.Error())
			return
		}
		//反序列化消息
		var message *Message
		err = json.Unmarshal(payload[:n], &message)

		println("<<<[INFO] 客户端读消息：", message.String())

		switch message.Type {
		case MsgData:
			// 过滤已经读过的消息
			//if message.SeqNum < c.seqNum {
				//把从服务器端读取到的消息塞到readChan中
			c.readChan <- message
			//读到服务器端的新dataMsg时，更新client端的seqNum
			if message.SeqNum > int(c.seqNum) {
				c.seqNum = int32(message.SeqNum)
			}
			//}
		case MsgAck:
			println("<<<[DEBUG] Client handle Read ACK...", message.String())
			//接收到ACK，解除超时警报
			c.timers[message.SeqNum].ackChan <- true
		}

	}
}

func (c *client) handleWrite() {
	for {
		select {
		case msg := <- c.writeChan:

			if c.alive {
				println("<<<[INFO] Client send message to server: ", msg.String())
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
				//写完消息，触发计时器
				if msg.Type == MsgData {
					println("<<<[INFO] 客户端开启计时器 ", msg.String())
					c.timers[msg.SeqNum] = &timer{
						timerChan:	make(chan *Message),
						ackChan:	make(chan bool),
						epochTimes:	0,
					}
					c.timers[msg.SeqNum].timerChan <- msg
					//初始化重试次数
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
	if msg.Type == MsgData {
		ack := NewAck(msg.ConnID, msg.SeqNum)
		c.writeChan <- ack
	}
}

/*
处理计时器
 */
func (c *client) handleTimer(timer *timer)  {
	for {
		select {
		case timerMsg := <- timer.timerChan:
			for {
				timeoutChan := time.After(time.Millisecond * time.Duration(c.params.EpochMillis))
				select {
				case <-timeoutChan:
					if timer.epochTimes < c.params.EpochLimit {
						println("<<<[DEBUG] Client", timer.epochTimes, "th timeout, resend")
						go c.resend(timerMsg)
					}else {
						//达到最大次数，判定连接断开
						println("<<<[DEBUG] Message timeout, exiting...")
						c.alive = false
						close(c.closeChan)
						return
					}
				case <- c.timers[timerMsg.SeqNum].ackChan:
					println("<<<[DEBUG] 客户端收到ack，计时器结束")
					//删除计时器
					delete(c.timers, timerMsg.SeqNum)
					return
				}
			}
		case <-c.closeChan:
			println("<<<[INFO]客户端关闭")
			return
		}
	}
}


/*
客户端处理超时的协程
 */
func (c *client) handleTimeout() {
	for{
		//对每一个DataMsg创建一个计时器，然后再用一个协程去不断监测是否超时
		for _, timer := range c.timers {
			if timer != nil {
				go c.handleTimer(timer)
			}
		}
		/*
		select {
		//开始计时
		//TODO 这里他妈的也有BUG，timerChan 不应该做成阻塞的，而是对于每个dataMsg都有一个timerChan
		case timerMsg := <- c.timer.timerChan:
			println("<<<[DEBUG] Client timer start:", timerMsg.String())
			timeoutChan := time.After(time.Millisecond * time.Duration(c.params.EpochMillis))
			select {
				//连接超时
				case <- timeoutChan:
					times, ok := c.timer.epochTimes[timerMsg.SeqNum]
					if ok && times < c.params.EpochLimit {
						println("<<<[DEBUG] Client", times, "th timeout, resend")
						go c.resend(timerMsg)
					}else {
						//达到最大次数，判定连接断开
						println("<<<[DEBUG] Message timeout, exiting...")
						c.alive = false
						close(c.closeChan)
						return
					}

				//TODO 这里有bug！！！
				//收到dataMsg的ack
				case ack := <- c.timer.timeoutChanMap[timerMsg.SeqNum]:
					println("<<<[DEBUG] 客户端收到ack ", ack.String(), "，计时器结束")
					continue
				}
		case <- c.closeChan:
			return
		}
		*/
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
	//epoch次数加一
	c.timers[msg.SeqNum].epochTimes = c.timers[msg.SeqNum].epochTimes + 1
	//再次加入计时器
	c.timers[msg.SeqNum].timerChan <- msg
}

func (c *client) atomicAddSeq()  {
	atomic.AddInt32(&c.seqNum, 1)
}