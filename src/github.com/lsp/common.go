package lsp

type timer struct {
	timerChan	chan *Message   //计时器channel
	ackChan		chan bool		//对于每一个消息都对应一个超时channel
	epochTimes	int				//记录每个消息的当前epoch次数
}

func HandleErr(err error) (Client, error) {
	if err != nil {
		println(err.Error())
		return nil, err
	}
	return nil, nil
}