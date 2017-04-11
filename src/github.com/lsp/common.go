package lsp

type timer struct {
	timerChan		chan *Message       	//计时器channel
	timeoutChanMap	map[int]chan *Message	//对于每一个消息都对应一个超时channel
	epochTimes		map[int]int				//记录每个消息的当前epoch次数
	connectChan		chan *Message			//连接消息的超时channel
}

func HandleErr(err error) (Client, error) {
	if err != nil {
		println(err)
		return nil, err
	}
	return nil, nil
}