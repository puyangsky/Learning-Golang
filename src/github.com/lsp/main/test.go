package main

import (
	"github.com/lsp"
	//"os"
	//"github.com/lspnet"
	//"strconv"
	//"time"
	"github.com/lspnet"
	"strconv"
)

func main()  {

	port := 20001
	_, err := lsp.NewServer(port, makeParams(1,1000,2))
	errorHandler(err)
	println("start server")

	//server.Write(1, []byte("hello"))

	//time.Sleep(1000* time.Second)


	hostport := lspnet.JoinHostPort("localhost", strconv.Itoa(port))
	cli, err := lsp.NewClient(hostport, makeParams(1,1000,2))
	errorHandler(err)
	println("start client")

	println(cli.ConnID())

	//server.Read()

	data := []byte("fuck")
	cli.Write(data)
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