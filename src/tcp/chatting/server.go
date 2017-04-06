package main

import (
	"net"
	"fmt"
	"bufio"
	"os"
	"util"
)

/*
  自己定义的连接对象
  conn 连接对象
  prefix 连接标识
 */
type myConn struct {
	conn net.Conn
	prefix string
}

func handleConn(myconn *myConn) {
	for {
		buf := make([]byte, 1024)

		n, err := myconn.conn.Read(buf[:])
		util.ErrHandler(err, "[x] Error on read: ")

		var msg= string(buf[0:n])

		fmt.Printf("[*] %s sent: %s\n", myconn.prefix, string(buf[0:n]))

		//当客户端发送bye时结束这段对话
		if msg == "bye" || msg == "" {
			fmt.Printf("[*] Exiting conversation with %s\n", myconn.prefix)
			break
		}

		ret := make([]byte, 1024)
		fmt.Print("[*] Please input return message: ")

		//读取用户输入作为返回值
		reader := bufio.NewReader(os.Stdin)
		ret, _, err = reader.ReadLine()

		util.ErrHandler(err, "[x] Error on input return message")

		if len(ret) != 0 {
			_, err = myconn.conn.Write(ret)
			util.ErrHandler(err, "[x] Error on sending return message")
			fmt.Printf("[*] Send return message to %s, msg: %s \n", myconn.prefix, ret)
		}
	}
	myconn.conn.Close()
}


func main() {
	//listen
	ln, err := net.Listen("tcp", ":20000")
	util.ErrHandler(err, "[x] Error on listening")

	fmt.Println("[*] Waiting for connection")

	//记录客户端连接的个数
	connNumber := 1

	for {

		conn, err := ln.Accept()
		util.ErrHandler(err, "[x] Error on accept")
		myconn := &myConn{
			conn:conn,
			prefix: fmt.Sprintf("[the %dth client]", connNumber),
		}

		//创建一个goroutine处理客户端连接
		go handleConn(myconn)

		connNumber++
	}

}
