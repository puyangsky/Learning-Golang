package main

import (
	"net"
	"strconv"
	"os"
)

const port  = 20001

func NewServer() {
	addr, _ := net.ResolveUDPAddr("udp4", ":" + strconv.Itoa(port))
	conn, _ := net.ListenUDP("udp4", addr)

	println("Server start at: ", "lcoalhost:" + strconv.Itoa(port))

	bytes := make([]byte, 1000)
	for {
		n, addr, err := conn.ReadFromUDP(bytes)
		if err != nil {
			os.Exit(1)
		}
		println(addr.String())
		println(string(bytes[0:n]))

		_, err = conn.WriteToUDP(bytes[0:n], addr)
		if err != nil {
			println(err.Error())
		}
	}
}

func main() {
	NewServer()
}