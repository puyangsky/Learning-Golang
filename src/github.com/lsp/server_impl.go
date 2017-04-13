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
	connID			int  				//客户端的编号
	alive			bool 				//该客户端是否在线
	closeChan		chan byte			//客户端关闭的channel
	addr			*lspnet.UDPAddr		//客户端的地址
	writeChan		chan *Message		//向客户端发送的消息缓冲区
	udpConn 		*lspnet.UDPConn 	//客户端连接
	timer			*timer				//计时器
	curSeqNum		int				//当前消息序号，用来去除已读消息
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

func addConnID() {
	atomic.AddInt32(&connID, 1)
}

// NewServer creates, initiates, and returns a new server. This function should
// NOT block. Instead, it should spawn one or more goroutines (to handle things
// like accepting incoming client connections, triggering epoch events at
// fixed intervals, synchronizing events using a for-select loop like you saw in
// project 0, etc.) and immediately return. It should return a non-nil error if
// there was an error resolving or listening on the specified port number.
func NewServer(port int, params *Params) (Server, error) {

	lspnet.EnableDebugLogs(true)

	udpAddr, err := lspnet.ResolveUDPAddr("udp4", ":" + strconv.Itoa(port))
	HandleErr(err)

	conn, err := lspnet.ListenUDP("udp4", udpAddr)
	HandleErr(err)
	println("[DEBUG] Server listen at :", port)

	//defer conn.Close()

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
		println("[INFO] Read message from client: ", msg.String())
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
		//更新客户端的消息序列
		_client.curSeqNum = int(s.seqNum)
		return nil
	}else{
		return errors.New("Invalid connID")
	}
}

func (s *server) CloseConn(connID int) error {
	if client, ok := s.mapping[connID]; ok {
		client.alive = false
		close(client.closeChan)
	}
	return nil
}

func (s *server) Close() error {
	close(s.closeChan)
	s.alive = false
	return nil
}



func (s *server) handleRead(msg *Message) {
	s.readChan <- msg
}


func (s *server) handleWrite(_client *clientInfo) {
	for {
		select {
		case msg := <- _client.writeChan:
			// TODO 注意msg的类型
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

					// 发送完消息
					// 注册计时器到客户端的timer中
					client.timer.timerChan <- msg
					client.timer.epochTimes[msg.SeqNum] = 0
				}
			} else {
				return
			}

		case <- s.closeChan:
			return
		}
	}
}

func (s *server)handleRequest(conn *lspnet.UDPConn) {
	for {
		var payload = make([]byte, 1000)
		n, addr, err := conn.ReadFromUDP(payload)
		println("read ", n, " byte")
		if err != nil {
			println(err.Error())
			return
		}

		var msg *Message
		err = json.Unmarshal(payload[0:n], &msg)
		if err != nil {
			println(err.Error())
			return
		}
		println("Get a connection,", addr.String(), msg.String())

		var _client *clientInfo

		_, exist := s.mapping[msg.ConnID]

		if msg.Type == MsgConnect && !exist {

			//检查是否已经创建了该连接，若已经创建，discard该消息
			_timer := &timer{
				timerChan:		make(chan *Message),
				timeoutChanMap: make(map[int]chan *Message),
				epochTimes:		make(map[int]int),
			}
			_client = &clientInfo{
				connID: 	 	int(connID),
				alive:		 	true,
				closeChan: 		make(chan byte),
				writeChan:	 	make(chan *Message, s.params.WindowSize),
				addr:	     	addr,
				udpConn:	 	conn,
				timer:			_timer,
				curSeqNum:		1,
			}
			//注册到服务器端的映射关系中
			s.mapping[_client.connID] = _client
			//ConnID自增
			addConnID()
			//发送ACK给客户端
			s.sendAck(_client, msg)
		}else {
			_client, ok := s.mapping[msg.ConnID]
			//不存在该客户端，或该消息已读，跳过
			if !ok || msg.SeqNum < _client.curSeqNum {
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
				_client.timer.timeoutChanMap[msg.SeqNum] = timeoutChan
			}
		}

		go s.handleWrite(_client)
	}
}

func (s *server) sendAck(_client *clientInfo, msg *Message) {
	if msg.Type == MsgConnect {
		ackToConnect := NewAck(_client.connID, _client.curSeqNum)
		_client.writeChan <- ackToConnect
	}else {
		ack := NewAck(_client.connID, msg.SeqNum)
		// 异步的方式发送
		_client.writeChan <- ack
	}
}

/*
处理超时的协程
*/
func (s *server) handleTimeout() {
	for {
		//遍历每个客户端
		for _, _client := range s.mapping {
			select {
			case timerMsg := <- _client.timer.timerChan:
				timeoutChan := time.After(time.Millisecond * time.Duration(s.params.EpochMillis))
				select {
					// 超时重发，最大次数时判定客户端连接挂了
					case <- timeoutChan:
						times, ok := _client.timer.epochTimes[timerMsg.SeqNum]
						//没达到最大epoch次数，触发重发机制
						if ok && times < s.params.EpochLimit {
							go _client.resend(timerMsg)
						}else {
							//达到最大次数，判定连接断开
							println("[DEBUG] Message Timeout, ConnID: ", timerMsg.ConnID, ", seqNum: ", timerMsg.SeqNum)
							_client.alive = false
							close(_client.closeChan)
							return
						}
					//TODO 未超时情况下，收到客户端的ACK
					case <- _client.timer.timeoutChanMap[timerMsg.SeqNum]:
						continue
					}
			case <- _client.closeChan:
				return
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

/*
服务器端重发消息
 */
func (c *clientInfo) resend(msg *Message)  {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_, err = c.udpConn.WriteToUDP(payload, c.addr)
	if err != nil {
		return
	}
	c.timer.timerChan <- msg
	//epoch次数加一
	c.timer.epochTimes[msg.SeqNum]++
}