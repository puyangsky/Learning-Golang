package main

import (
	"github.com/lsp"
	"os"
)

type c struct {
	id int
}

func main()  {

	_, err := lsp.NewServer(2000, makeParams(1,1000,1))
	errorHandler(err)
	println("start server")

	cli, err := lsp.NewClient("localhost:2000", makeParams(1,1000,1))
	errorHandler(err)
	println("start client")

	println(cli.ConnID())

	var ms *lsp.Message
	if ms == nil {
		println("nil")
	}
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
		println(err)
		os.Exit(-1)
	}
}