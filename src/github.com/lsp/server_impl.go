// Contains the implementation of a LSP server.

package lsp

import (
	"errors"
	"sync/atomic"
	"github.com/lspnet"
	"strconv"
	"encoding/json"
)

type clientInfo struct {
	connID		int  			//客户端的编号
	alive		bool 			//该客户端是否在线
	addr		*lspnet.UDPAddr	//客户端的地址
	writeChan	chan *Message
	udpConn 		*lspnet.UDPConn
}


type server struct {
	seqNum		int
	//udpConn   	*lspnet.UDPConn
	readChan	chan *Message
	mapping		map[int]*clientInfo //clientInfo与ConnID之间的映射关系
	closeChan	chan byte
	alive		bool
}

var connID int32 = 0

func addConnID() int32 {
	return atomic.AddInt32(&connID, 1)
}

// NewServer creates, initiates, and returns a new server. This function should
// NOT block. Instead, it should spawn one or more goroutines (to handle things
// like accepting incoming client connections, triggering epoch events at
// fixed intervals, synchronizing events using a for-select loop like you saw in
// project 0, etc.) and immediately return. It should return a non-nil error if
// there was an error resolving or listening on the specified port number.
func NewServer(port int, params *Params) (Server, error) {
	udpAddr, err := lspnet.ResolveUDPAddr("udp", "localhost:" + strconv.Itoa(port))
	HandleErr(err)

	conn, err := lspnet.ListenUDP("udp", udpAddr)
	HandleErr(err)

	println("[DEBUG]Server listen at localhost:", port)

	server := &server{
		seqNum:		0,
		//udpConn:	conn,
		readChan:	make(chan *Message),
		mapping:	make(map[int]*clientInfo),
		closeChan:	make(chan byte),
		alive:		true,
	}

	//TODO：把读操作移到handleRead方法中做
	go func() {
		for {
			var payload = make([]byte, 1000)
			_, addr, err := conn.ReadFromUDP(payload)
			if err != nil {
				return
			}
			var msg *Message
			err = json.Unmarshal(payload, msg)
			if err != nil {
				return
			}

			_clientInfo := &clientInfo{
				connID: int(connID),
				alive:	true,
				addr:	addr,
				udpConn:	conn,
			}

			//客户端申请的连接请求
			if msg.Type == MsgConnect {

				//ID自增
				addConnID()
				server.mapping[_clientInfo.connID] = _clientInfo
			}

			if msg.Type == MsgData {
				server.handleRead(_clientInfo)
				server.handleWrite(_clientInfo)
			}



		}
	}()

	return server, nil
}

func (s *server) Read() (int, []byte, error) {
	select {
	case msg := <- s.readChan:
		println("[INFO] Send message to client: ", msg.String())
		return msg.ConnID, msg.Payload, nil
	case <- s.closeChan:
		return -1, nil, nil
	} // Blocks indefinitely.
}

func (s *server) Write(connID int, payload []byte) error {
	if _client, ok := s.mapping[connID]; ok {
		message := NewData(connID, s.seqNum, payload)
		s.seqNum++
		//把消息塞到writeChan中
		_client.writeChan <- message
		return nil
	}else{
		return errors.New("Invalid connID")
	}
}

func (s *server) CloseConn(connID int) error {
	if client, ok := s.mapping[connID]; ok {
		client.alive = false
	}
	return errors.New("Invalid connID")
}

func (s *server) Close() error {
	return errors.New("not yet implemented")
}



func (s *server) handleRead(_client *clientInfo) {
	for {
		var payload = make([]byte, 1000)
		_, err := _client.udpConn.Read(payload)
		if err != nil {
			return
		}

		//反序列化
		var message *Message
		err = json.Unmarshal(payload, message)
		if err != nil {
			return
		}
	}
}



func (s *server) handleWrite(_client *clientInfo) {
	for {
		select {
		case msg := <- _client.writeChan:
			bytes, err := json.Marshal(msg)
			if err != nil {
				return
			}

			mConnID := msg.ConnID
			if client, ok := s.mapping[mConnID]; ok {
				if s.alive && client.alive {
					//真正写消息的地方
					_, err = _client.udpConn.WriteToUDP(bytes, client.addr)
					if err != nil{
						return
					}
					println("[INFO] Send message to client: ", msg.String())
				}
			} else {
				return
			}

		case <- s.closeChan:
			return
		}
	}
}