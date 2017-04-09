package lsp

import (
	"github.com/lspnet"
	"encoding/json"
	"errors"
)

type client struct {
	hostport 	string
	params   	*Params
	udpconn  	*lspnet.UDPConn
	connID   	int // 客户端的连接ID
	readChan 	chan *Message
	writeChan	chan *Message
	closeChan	chan byte
	alive		bool
	seqNum		int // 消息序列号，只有发送data message时才增长
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

func HandleErr(err error) (Client, error) {
	if err != nil {
		println(err)
		return nil, err
	}
	return nil, nil
}

func NewClient(hostport string, params *Params) (Client, error) {
	udpAddr, err := lspnet.ResolveUDPAddr("udp", hostport)
	HandleErr(err)

	udpConn, err := lspnet.DialUDP("udp4", nil, udpAddr)
	HandleErr(err)

	//创建连接请求消息
	connectMsg := NewConnect()
	//序列化
	payload, err := json.Marshal(connectMsg)
	HandleErr(err)

	//发送消息
	_, err = udpConn.Write(payload)
	HandleErr(err)

	//读取服务器端返回的消息
	var readPayload = make([]byte, 512)
	_, err = udpConn.Read(readPayload)
	HandleErr(err)
	//反序列化
	var ackMsg Message
	err = json.Unmarshal(readPayload, &ackMsg)
	HandleErr(err)

	client := &client{
		hostport: hostport,
		params:   params,
		udpconn:  udpConn,
		connID:   ackMsg.ConnID,
		readChan: make(chan *Message),
		writeChan:make(chan *Message, params.WindowSize),
		alive:	  true,
	}
	go client.handleRead()
	go client.handleWrite()
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
		println("[INFO] Receive message from client: ", string(msg))
		//读完消息后要返回一个ACK消息给服务器端
		ackMsg := NewAck(c.connID, msg.SeqNum)
		c.writeChan <- ackMsg
		return msg.Payload, nil
	//TODO：处理超时的情况
	//case <-timeout:

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
	err := c.udpconn.Close()
	return err
}

//用goroutine来不断监听读事件
func (c *client) handleRead() {
	for{
		payload := make([]byte, 2000)
		_, err := c.udpconn.Read(payload)
		if err != nil {
			return
		}
		//反序列化消息
		var message *Message
		err = json.Unmarshal(payload, &message)

		//把从服务器端读取到的消息塞到readChan中
		c.readChan <- message
	}
}

func (c *client) handleWrite() {
	for {
		select {
		case msg := <- c.writeChan:
			if c.alive {
				println("[INFO] Send message to server: ", msg.String())
				//序列化消息
				bytes, err := json.Marshal(msg)
				if err != nil {
					continue
				}
				//真正写消息的地方
				c.udpconn.Write(bytes)
				//TODO：写完消息要触发计时器，如果超时没收到ACK要重新发送消息

			}
		case <-c.closeChan:
			return
		}
	}
}


func (c *client) FireEpoch() {

}