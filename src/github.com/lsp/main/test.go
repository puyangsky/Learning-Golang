package main

import (
	"github.com/lsp"
	//"os"
	//"github.com/lspnet"
	//"strconv"
	//"time"
	"github.com/lspnet"
	"strconv"
	//"time"
	//"time"
)

func main()  {

	port := 20001
	server, err := lsp.NewServer(port, makeParams(1,1000,2))
	errorHandler(err)
	println("start server")

	//server.Write(1, []byte("hello"))

	//time.Sleep(1000* time.Second)


	hostport := lspnet.JoinHostPort("localhost", strconv.Itoa(port))
	cli, err := lsp.NewClient(hostport, makeParams(1,1000,2))
	errorHandler(err)
	println("start client")

	data := []byte("fuck")
	cli.Write(data)
	server.Read()

	//time.Sleep(5*time.Second)
	println("\n=================================================================")

	data = []byte("shit")
	server.Write(cli.ConnID(), data)
	cli.Read()

	//time.Sleep(2*time.Second)

}

func makeParams(epochLimit, epochMillis, windowSize int) *lsp.Params {
	return &lsp.Params{
		EpochLimit:  epochLimit,
		EpochMillis: epochMillis,
		WindowSize:  windowSize,
	}
}

func errorHandler(err error) {
	if err != nil {
		println("err:", err.Error())
		//os.Exit(-1)
	}
}