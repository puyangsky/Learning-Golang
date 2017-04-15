// Contains the implementation of a LSP server.

package lsp

import (
	//"errors"
	"sync/atomic"
	"github.com/lspnet"
	"strconv"
	"encoding/json"
	"time"
	//"sync"
)


type clientInfo struct {
	connID			int  				//客户端的编号
	alive			bool 				//该客户端是否在线
	closeChan		chan byte			//客户端关闭的channel
	addr			*lspnet.UDPAddr		//客户端的地址
	writeChan		chan *Message		//向客户端发送的消息缓冲区
	udpConn 		*lspnet.UDPConn 	//客户端连接
	timers			[]*timer		//计时器
	curSeqNum		int32				//当前消息序号，用来去除已读消息
}


type server struct {
	readChan	chan *Message
	clients		[]*clientInfo //clientInfo与server之间的映射关系
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
	println(">>>[DEBUG] Server listen at :", port)

	//defer conn.Close()

	server := &server{
		readChan:	make(chan *Message),
		clients:	make([]*clientInfo, 0, 0),
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
		println(">>>[INFO] Read message from client: ", msg.String())
		//真正读的地方
		return msg.ConnID, msg.Payload, nil
	case <- s.closeChan:
		return -1, nil, nil
	} // Blocks indefinitely.
}

func (s *server) Write(connID int, payload []byte) error {
	_client := s.clients[connID]
	message := NewData(connID, int(_client.curSeqNum), payload)
	//更新客户端的消息序列
	_client.atomicAddSeq()
	//把消息塞到writeChan中
	_client.writeChan <- message
	return nil

}

func (s *server) CloseConn(connID int) error {
	client := s.clients[connID]
	client.alive = false
	close(client.closeChan)
	return nil
}

func (s *server) Close() error {
	close(s.closeChan)
	s.alive = false
	for _, c := range s.clients {
		c.alive = false
		close(c.closeChan)
	}
	return nil
}


func (s *server) handleRead(msg *Message) {
	s.readChan <- msg
}


func (s *server) handleWrite(_client *clientInfo) {
	for {
		select {
		case msg, ok := <- _client.writeChan:
			if !ok {
				return
			}
			bytes, err := json.Marshal(msg)
			if err != nil {
				return
			}

			mConnID := msg.ConnID
			client := s.clients[mConnID]
			if s.alive && client.alive {
				//真正写消息的地方
				_, err = _client.udpConn.WriteToUDP(bytes, client.addr)
				if err != nil{
					return
				}
				println(">>>[INFO] Server send message to client: ", msg.String(), client.addr.String())
				if msg.Type == MsgData {
					// 发送完消息，注册计时器到客户端的timer中
					timer := &timer{
						timerChan:	make(chan *Message),
						ackChan:	make(chan int),
						epochTimes:	0,
						//lock:		&sync.Mutex{},
					}
					println(">>>[INFO] 服务器端开启计时器:", msg.String())
					client.timers = append(client.timers, timer)
					println("服务器端：",len(client.timers), msg.SeqNum)
					client.timers[msg.SeqNum - 1].timerChan <- msg
				}
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
		println(">>>[INFO] Server get a connection,", addr.String(), msg.String())

		//client := s.clients[msg.ConnID]

		if msg.Type == MsgConnect {

			//检查是否已经创建了该连接，若已经创建，discard该消息
			//timer := &timer{
			//	timerChan:		make(chan *Message),
			//	ackChan: 		make(chan bool),
			//	epochTimes:		0,
			//}
			_client := &clientInfo{
				connID: 	 	int(connID),
				alive:		 	true,
				closeChan: 		make(chan byte),
				writeChan:	 	make(chan *Message, s.params.WindowSize),
				addr:	     	addr,
				udpConn:	 	conn,
				timers:			make([]*timer, 0, 0),
				curSeqNum:		1,
			}
			//注册到服务器端的映射关系中
			s.clients = append(s.clients, _client)
			//ConnID自增
			addConnID()
			//发送ACK给客户端
			s.sendAck(_client, msg)
			go s.handleWrite(_client)

		}else {
			_client := s.clients[msg.ConnID]
			//该消息已读，跳过
			//if msg.SeqNum < int(_client.curSeqNum) {
			//	println(">>>[WARN] Discard message,", msg.String())
			//	continue
			//}

			if msg.Type == MsgData {
				//读到dataMsg时更新client的seqNum
				//_client.curSeqNum = int32(msg.SeqNum)

				go s.handleRead(msg)
				//发送ACK给客户端
				go s.sendAck(_client, msg)

			} else if msg.Type == MsgAck {
				//if timer, ok := _client.timers[msg.SeqNum-1]; ok {
				timer := _client.timers[msg.SeqNum - 1]
					//接收到ACK，解除超时警报
				println(">>>[DEBUG] Server 接收到 ACK", msg.String())
				timer.ackChan <- msg.SeqNum
				//}

			}
			go s.handleWrite(_client)
		}
	}
}

func (s *server) sendAck(_client *clientInfo, msg *Message) {
	// 异步的方式发送
	if msg.Type == MsgConnect {
		ackToConnect := NewAck(_client.connID, int(_client.curSeqNum))
		_client.writeChan <- ackToConnect
	}else {
		ack := NewAck(_client.connID, msg.SeqNum)
		_client.writeChan <- ack
	}
}

/*
处理计时器
 */
func (s *server) handleTimer(client *clientInfo, timer *timer)  {
	for {
		select {
		case timerMsg := <- timer.timerChan:
			for {
				timeoutChan := time.After(time.Millisecond * time.Duration(s.params.EpochMillis))
				select {
				case <-timeoutChan:
					if timer.epochTimes < s.params.EpochLimit {
						println(">>>[DEBUG] Server", timer.epochTimes, "th timeout, resend")
						go client.resend(timerMsg)
					}else {
						//达到最大次数，判定连接断开
						println(">>>[DEBUG] Server message timeout, exiting")
						client.alive = false
						close(client.closeChan)
						return
					}
				case seq := <- timer.ackChan:
					println(">>>[DEBUG] 服务器端收到ack，计时器结束, seq:",seq)
					// TODO: 使用一个doneChan来告诉计时器结束
					client.timers[seq - 1] = nil
				}
			}
		case <-client.closeChan:
			println(">>>[INFO]服务器端关闭")
			return
		}
	}
}


/*
处理超时的协程
*/
func (s *server) handleTimeout() {
	println(">>>[INFO] 处理超时，", len(s.clients))
	//time.Sleep(1*time.Second)
	for {
		//遍历每个客户端
		for _, _client := range s.clients {
			for _, timer := range _client.timers {
				if timer != nil {
					go s.handleTimer(_client, timer)
				}
			}
		}
	}
}
/*
线程安全地自增序列号
 */
func (c *clientInfo) atomicAddSeq() {
	atomic.AddInt32(&c.curSeqNum, 1)
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
	timer := c.timers[msg.SeqNum-1]
	//timer.lock.Lock()
	//epoch次数加一
	timer.epochTimes = timer.epochTimes + 1
	//再次加入计时器
	timer.timerChan <- msg
	//timer.lock.Unlock()
}