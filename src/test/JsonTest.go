package main

import "encoding/json"


type T struct {
	A int
	B string
}


func main()  {
	t := &T{
		A:1,
		B:"a",
	}
	b, _ := json.Marshal(t)
	println(string(b))

	var c *T
	err := json.Unmarshal(b, &c)
	if err != nil {
		println("err:", err.Error())
	}
	if c == nil {
		print("nil")
	}
	println(c.A, c.B)

	a := make([]int, 0, 0)
	println(len(a))
	a = append(a, 1)
	println(len(a))
	println(a[len(a)-1])
	a = append(a, 2)
	println(len(a))

	for k, v := range a {
		println(k, v)
	}
}
