// Implementation of a MultiEchoServer. Students should write their code in this file.

package p0

import (
	"net"
	"fmt"
	"bufio"
)

const DEFAULT_POOL_SIZE  = 100

type clientInfo struct {
	live			bool 		//是否存活
	conn			net.Conn	//连接
	writeBuffers	chan string	//写缓冲区
}

type multiEchoServer struct {
	count   	int 		//连接数
	clients		[]*clientInfo  //客户端连接数组
	readChan	chan string	//收到的所有消息
	closeChan	chan byte
}


// New creates and returns (but does not start) a new MultiEchoServer.
func New() MultiEchoServer {
	mes := &multiEchoServer{
		count:		0,
		clients:	make([]*clientInfo, 0, 0),
		readChan:	make(chan string),
		closeChan:	make(chan byte),
	}
	go mes.handleBroadcast()
	return mes
}

func (mes *multiEchoServer) Start(port int) error {
	ln, err :=net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	//println("[DEBUG] Server start listen at: ", fmt.Sprintf("localhost:%d", port))

	if err != nil {
		return err
	}
	go func() {
		for{
			conn, err := ln.Accept()
			if err != nil {
				continue
			}

			//println("[DEBUG] Accept a client request.")
			mes.count++
			client := &clientInfo{
				live:			true,
				conn:			conn,
				writeBuffers:	make(chan string, DEFAULT_POOL_SIZE),
			}
			mes.clients = append(mes.clients, client)
			go mes.handleRead(client)
			go mes.handleWrite(client)
		}
	}()

	return nil
}

func (mes *multiEchoServer) Close() {
	//???
	close(mes.closeChan)

	for _, client :=  range mes.clients {
		if client.live {
			client.conn.Close()
		}
	}
}

func (mes *multiEchoServer) Count() int {
	return mes.count
}

func (mes *multiEchoServer) handleBroadcast() {
	for {
		select {
		case message := <- mes.readChan:
			for _, client := range mes.clients {
				//把消息塞到每个客户端中
				if client.live && len(client.writeBuffers) < DEFAULT_POOL_SIZE {
					client.writeBuffers <- message
				}
			}
		case <- mes.closeChan:
			return
		}
	}
}

// 按行分割，将读到的消息都塞到readChan中
func (mes *multiEchoServer) handleRead(client *clientInfo) {
	reader := bufio.NewReader(client.conn)
	for{
		message, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}

		//println("[INFO] Receive message from client: ", string(message))
		mes.readChan <- string(message)
	}
}


//把该客户端中的写缓冲区中的消息都发送给客户端
func (mes *multiEchoServer) handleWrite(client *clientInfo) {
	for {
		select {
		case message := <- client.writeBuffers:
			if client.live {
				client.conn.Write([]byte(message))
				//println("[INFO] Send message to client: ", string(message))
			}
		case <- mes.closeChan:
			return
		}
	}
}


