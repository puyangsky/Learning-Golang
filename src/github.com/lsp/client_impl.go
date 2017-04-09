package lsp

import (
	"github.com/lspnet"
	"encoding/json"
	"time"
	"errors"
)

type client struct {
	hostport string
	params   *Params
	udpconn  *lspnet.UDPConn
	connID   int
	seqNum   int
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

func handleErr(err error) (Client, error) {
	if err != nil {
		println(err)
		return nil, err
	}
	return nil, nil
}

func NewClient(hostport string, params *Params) (Client, error) {
	udpAddr, err := lspnet.ResolveUDPAddr("udp", hostport)
	handleErr(err)

	udpConn, err := lspnet.DialUDP("udp4", nil, udpAddr)
	handleErr(err)

	//创建连接请求消息
	connectMsg := NewConnect()
	//序列化
	payload, err := json.Marshal(connectMsg)
	handleErr(err)

	//发送消息
	_, err = udpConn.Write(payload)
	handleErr(err)

	//读取服务器端返回的消息
	var readPayload = make([]byte, 2000)
	_, err = udpConn.Read(readPayload)
	handleErr(err)
	//反序列化
	var ackMsg Message
	err = json.Unmarshal(readPayload, &ackMsg)
	handleErr(err)

	return &client{
		hostport: hostport,
		params:   params,
		udpconn:  udpConn,
		connID:   ackMsg.ConnID,
		seqNum:   0,
	}, nil
}

func (c *client) ConnID() int {
	return c.connID
}

func (c *client) Read() ([]byte, error) {
	timeout := make(chan bool)
	done := make(chan bool)
	payload := make([]byte, 2000)

	go func() {
		_, err := c.udpconn.Read(payload)
		if err != nil {
			println(err)
		}
		done <- true
	}()

	go func() {
		for epochLimit := 0; epochLimit < c.params.EpochLimit; epochLimit++ {
			time.Sleep(2 * time.Second)
		}
		timeout <- true
	}()

	select {
	case <- done:
		//接收服务器端的数据完毕后，向服务器端发送ack
		ackMsg := NewAck(c.connID, c.seqNum)
		bytes, err := json.Marshal(ackMsg)
		if err != nil {
			println(err)
			return nil, err
		}
		err = c.Write(bytes)
		if err != nil {
			println(err)
			return nil, err
		}
		c.seqNum++
		return payload, nil
	//超时处理
	case <- timeout:
		return nil, errors.New("[Error] Read timeout!")
	}
}

//发送消息
func (c *client) Write(payload []byte) error {
	//TODO：如果没有收到ack，则重新发送
	go func() error {
		_, err := c.udpconn.Write(payload)
		if err != nil {
			println(err)
			return err
		}

		var read = make([]byte, 2000)
		_, err = c.udpconn.Read(read)
		if err != nil {
			println(err)
			return err
		}

		var ackMsg Message
		err = json.Unmarshal(read, &ackMsg)
		if err != nil {
			println(err)
			return err
		}
		if ackMsg{

		}

		return nil
	}()
	return nil
}

func (c *client) Close() error {
	err := c.udpconn.Close()
	if err != nil {
		return err
	}
	return nil
}