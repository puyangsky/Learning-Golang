package main

import "net"

func NewClient() *net.UDPConn {
	addr, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:20001")
	conn, _ := net.DialUDP("udp4", nil, addr)
	return conn
}

func sendMsg(conn *net.UDPConn) {
	msg := make([]byte, 1000)
	msg = []byte("fuckyou")
	n, err := conn.Write(msg)
	println("Client send msg to server,", string(msg))
	if err != nil {
		println(err.Error())
	}

	n, err = conn.Read(msg)
	println("Client get msg from server,", string(msg[0:n]))
	if err != nil {
		println(err.Error())
	}
}

func main() {
	sendMsg(NewClient())
}
