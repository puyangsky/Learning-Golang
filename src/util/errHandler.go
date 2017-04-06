package util

import (
	"fmt"
	//"os"
)

func ErrHandler(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %s\n", msg, err)
		//os.Exit(-1)
	}
}
