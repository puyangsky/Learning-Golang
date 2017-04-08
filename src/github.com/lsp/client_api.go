package lsp

//客户端API
type Client interface {
	//返回连接的ID
	ConnID() int

	//从服务器端的socket中读取数据，并返回byte数组
	Read() ([]byte, error)

	//将payload数据写到socket，发送给服务器端
	Write(payload []byte) error

	//关闭连接
	Close() error
}

