package lsp

import "fmt"

//定义消息类型
type MsgType int

const (
	MsgConnect MsgType = iota  // Sent by clients to make a connection w/ the server.
	MsgData                    // Sent by clients/servers to send data.
	MsgAck                     // Sent by clients/servers to ack connect/data msgs.
)


//定义消息结构体
type Message struct {
	Type    MsgType
	ConnID  int
	SeqNum  int
	Payload []byte
}


//创建一个连接请求
func NewConnect() *Message {
	return &Message{Type: MsgConnect}
}

//创建一个数据消息
func NewData(connID int, seqNum int, payload []byte) *Message {
	return &Message{
		Type: MsgConnect,
		ConnID: connID,
		SeqNum: seqNum,
		Payload: payload,
	}
}

//创建一个确认消息
func NewAck(connID, seqNum int) *Message {
	return &Message{
		Type:   MsgAck,
		ConnID: connID,
		SeqNum: seqNum,
	}
}


func (m *Message) String() string {
	var name, payload string
	switch m.Type {
	case MsgConnect:
		name = "Connect"
	case MsgData:
		name = "Data"
		payload = " " + string(m.Payload)
	case MsgAck:
		name = "Ack"
	}
	return fmt.Sprintf("[%s %d %d%s]", name, m.ConnID, m.SeqNum, payload)
}