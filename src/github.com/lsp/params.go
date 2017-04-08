package lsp

import "fmt"

//参数默认大小
const (
	DefaultEpochLimit  = 5
	DefaultEpochMillis = 2000
	DefaultWindowSize  = 1
)

//参数结构体
type Params struct {
	EpochLimit int
	EpochMills int
	WindowSize int
}


//返回默认值
func NewParams() *Params {
	return &Params{
		EpochLimit: DefaultEpochLimit,
		EpochMills: DefaultEpochMillis,
		WindowSize: DefaultWindowSize,
	}
}


//toString
func(p *Params) String() string {
	return fmt.Sprintf("[EpochLimit: %d, EpochMillis: %d, WindowSize: %d]",
		p.EpochLimit, p.EpochMills, p.WindowSize)
}