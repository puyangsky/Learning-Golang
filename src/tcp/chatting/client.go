package main

import (
	"net"
	"fmt"
	"bufio"
	"os"
	"util"
)

const ADDRESS  = "localhost:20000"

func main() {
	conn, err := net.Dial("tcp", ADDRESS)
	defer conn.Close()
	util.ErrHandler(err, "[x] Error on Dial")

	fmt.Printf("[*] Connecting to server at %s\n", ADDRESS)

	msg := make([]byte, 1024)

	fmt.Println("[*] Sending something to server")
	for{
		fmt.Print("[*] Please input something: ")
		reader := bufio.NewReader(os.Stdin)
		msg, _, err = reader.ReadLine()
		util.ErrHandler(err, "[x] Error on Input")

		conn.Write(msg)
		fmt.Printf("[*] Send a msg: %s\n", string(msg))

		//当发送的消息为'bye'时推出循环，结束连接
		if string(msg) == "bye" || string(msg) == ""{
			fmt.Println("[*] Exiting")
			break
		}

		//创建byte数组来读取服务器返回的消息
		buf := make([]byte, 1024)
		n, err := conn.Read(buf[:])
		util.ErrHandler(err, "[x] Error on receiving msg from server")

		if n != 0 {
			fmt.Printf("[*] Accept msg from server: %s\n", string(buf[0:n]))
		}
	}

	conn.Close()
}
