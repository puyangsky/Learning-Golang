// Contains the implementation of a LSP server.

package lsp

import (
	"errors"
	"sync/atomic"
	"github.com/lspnet"
	"strconv"
	"encoding/json"
	"time"
)

type clientInfo struct {
	connID			int  					//客户端的编号
	alive			bool 					//该客户端是否在线
	addr			*lspnet.UDPAddr			//客户端的地址
	writeChan		chan *Message			//向客户端发送的消息缓冲区
	udpConn 		*lspnet.UDPConn 		//客户端连接
	timerChan		chan *Message			//计时器channel
	timeoutChanMap	map[int]chan *Message	//超时channel
}


type server struct {
	seqNum		int32
	readChan	chan *Message
	mapping		map[int]*clientInfo //clientInfo与server之间的映射关系
	closeChan	chan byte
	alive		bool
	params		*Params
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

	lspnet.EnableDebugLogs(true)

	udpAddr, err := lspnet.ResolveUDPAddr("udp", "localhost:" + strconv.Itoa(port))
	HandleErr(err)

	conn, err := lspnet.ListenUDP("udp", udpAddr)
	HandleErr(err)

	println("[DEBUG]Server listen at localhost:", port)

	server := &server{
		seqNum:		0,
		readChan:	make(chan *Message),
		mapping:	make(map[int]*clientInfo),
		closeChan:	make(chan byte),
		alive:		true,
		params:		params,
	}

	go server.handleRequest(conn)
	go server.handleTimeout()
	return server, nil
}

func (s *server) Read() (int, []byte, error) {
	select {
	case msg := <- s.readChan:
		println("[INFO] Send message to client: ", msg.String())
		//真正读的地方
		return msg.ConnID, msg.Payload, nil
	case <- s.closeChan:
		return -1, nil, nil
	} // Blocks indefinitely.
}

func (s *server) Write(connID int, payload []byte) error {
	if _client, ok := s.mapping[connID]; ok {
		message := NewData(connID, int(s.seqNum), payload)
		s.atomicAddSeq()
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



func (s *server) handleRead(msg *Message) {
	s.readChan <- msg
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

					//发送完消息要注册超时计时器到客户端的timeoutChan中
					client.timerChan <- msg

				}
			} else {
				return
			}

		case <- s.closeChan:
			return
		}
	}
}

func (s *server)handleRequest(conn *lspnet.UDPConn)  {
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

		var _client *clientInfo

		if msg.Type == MsgConnect {
			_client = &clientInfo{
				connID: 	 	int(connID),
				alive:		 	true,
				writeChan:	 	make(chan *Message, s.params.WindowSize),
				addr:	     	addr,
				udpConn:	 	conn,
				timerChan:		make(chan *Message),
				timeoutChanMap: make(map[int]chan *Message),
			}
			s.mapping[_client.connID] = _client
			//ConnID自增
			addConnID()
			//发送ACK给客户端
			go s.sendAck(_client, msg)
		}else {
			_client, ok := s.mapping[msg.ConnID]
			if !ok {
				continue
			}

			if msg.Type == MsgData {
				go s.handleRead(msg)
				//发送ACK给客户端
				go s.sendAck(_client, msg)
			} else if msg.Type == MsgAck {
				//接收到ACK，解除超时警报
				var timeoutChan = make(chan *Message)
				timeoutChan <- msg
				_client.timeoutChanMap[msg.SeqNum] = timeoutChan
			}
		}

		go s.handleWrite(_client)
	}
}

func (s *server) sendAck(_client *clientInfo, msg *Message) error {
	ack := NewAck(_client.connID, msg.SeqNum)
	payload, err := json.Marshal(ack)
	if err != nil {
		return err
	}
	_, err = _client.udpConn.WriteToUDP(payload, _client.addr)
	return err
}

/*
TODO 处理超时的协程
*/
func (s *server) handleTimeout() error {
	for {
		//遍历每个客户端
		for _, _client := range s.mapping {
			select {
			case timerMsg := <- _client.timerChan:
				select {
					//TODO 超时重发，最大次数时判定客户端连接挂了
					case time.After(2*time.Second):
						return errors.New("Message Timeout")
					case <-_client.timeoutChanMap[timerMsg.SeqNum]:
						continue
					}
			}
		}
	}

}
/*
线程安全地自增序列号
 */
func (s *server) atomicAddSeq() {
	atomic.AddInt32(&s.seqNum, 1)
}