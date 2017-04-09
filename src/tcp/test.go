package main

import (
	"github.com/lspnet"
	"time"
)

type c struct {
	id int
}

func main()  {
	host, port, err := lspnet.SplitHostPort("localhost:8080")
	if err != nil {
		println(err)
	}
	println(host, port)

	//lspnet.DialUDP()

	myC := make(chan c)

	go func() {
		myC <- c{id:1}
	}()

	select {
	case cc := <- myC:
		println("cc:", cc.id)
	}


	//新建计时器，两秒以后触发，go触发计时器的方法比较特别，就是在计时器的channel中发送值

	timer1 := time.NewTimer(time.Second * 2)

	//此处在等待channel中的信号，执行此段代码时会阻塞两秒

	<-timer1.C

	println("Timer 1 expired")

}