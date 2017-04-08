# CMU440
CMU440是CMU的一个分布式课程，我学习的目的是因为想用go语言写一些项目，而这个课程的project正好是用golang来完成的。

## Project 1: Distributed Bitcoin Miner

### Part A: 实现LSP协议

#### 1 LPS Protocol

LSP Protocol(Live Sequence Protocol)是一个不同于TCP和UDP的协议，支持以下几点特征：

- 支持client-server通信模型
- 服务器与多个客户端保持连接，每个连接有唯一的标识ID
- 服务器与客户端之间的通信消息由一系列不相关的消息组成
- 消息在UDP包范围之内（约1000字节）
- 保证可靠性：每个包之内被接收一次，而且保证接收的顺序
- 服务器和客户端能够监测它们连接的状态，并且能够发现连接失效的情况

##### 1.1	LSP Overview

LSP连接每次发送的消息由以下部分组成：

- **Message Type**：用来标识发送的类型：0代表建立连接；1代表发送数据；2代表ack
- **Connection ID**:一个唯一的正整数标识连接的ID
- **Sequence Number**:一个正整数，用来标记一个连接中的每个包的序号，从0开始
- **Payload**:有效负载，发送的消息的主体，使用字节传输（建立连接和ack时payload为nil）

下面是几个例子：

- (Connection, 0, 0):连接请求，ID为0，序列号为0 
- (Data, *id*, *sn*, D):数据消息，ID为*id*，序列号为*sn*，负载为D
- (Ack, *id*, *sn*):Ack消息，ID为*id*，序列号为*sn*

#### 1.1.1 建立连接
建立连接的过程如图1。

![](http://images2015.cnblogs.com/blog/756310/201704/756310-20170407231123832-2013179783.png)


客户端发送建立连接请求到服务器端，服务器端会返回一个ACK，包含一个全局唯一的ID来标识这个连接。

##### 1.1.2 发送&接收数据

图2显示了LSP协议中发送和接收数据的示例，客户端向服务器端发送数据，服务器端会返回一个ack，同理服务器端也可以向客户端发送消息，道理类似。

![](http://images2015.cnblogs.com/blog/756310/201704/756310-20170407235606613-1063290263.png)

和TCP协议已于，LSP也包含滑动窗口协议，但是跟TCP的不太一样的是，LSP中的滑动窗口代表了当前可以发送的消息的数量的上限，我们使用*w*来标记滑动窗口数。例如，当*w*=1时，代表下一个消息发送之前，前一个消息必须被确认；当*w*=2时，最多可以有两个消息可以在未收到ack的情况下被发送。

我们假设当前未被确认的最后一个消息的序列号为*n*，此时滑动窗口值为*w*，那么只有序列号在*n*和*n+w-1*之间的消息可以被发送。由于LSP支持全双工，所以服务器端和客户端都需要保持一个各自的滑动窗口值。

##### 1.1.3 Epoch events

`注：Epoch events的翻译是划时代的事件，然而在文中并非这个含义，因此这里就用原文了`

从1.3中可以看出LSP的发送和接收数据过程并不满足健壮性，因为如果在发送过程中丢包了，那么就无法保证可靠性，并且会导致发送方一直等待ack。因此加入了超时机制。每次发送一个包时计时器就开始计时，我们把一次发送和接收的过程叫做一个*epoch*（纪元？）。计时器的超时时间默认为2000ms，超时时就会触发警报。

当计时器触发时，客户端会进行以下操作：

- 如果客户端的连接请求没有被服务器ack，则重新发送连接请求（可能是连接请求丢包）
- 如果连接请求已经被ack，但是没有收到数据消息，则重新发送ack确认消息，系列号为0（可能是ack确认消息丢包）
- 对于每个未被服务器端ack的数据消息，重新发送数据消息（数据消息丢包）
- 对于最后*w*个已ack了数据消息，重新发送ack消息（暂时还没弄明白为什么要重新发送ack）

服务器端同理。

图3展示了*epoch event*的示例，其中椭圆形的黑色区域就是一次*epoch event*。可以看到客户端向服务器端发送了一个数据消息，数据为*Di*，但是服务器端返回的ack丢失了。此外，服务器端想发送数据消息，数据为*Dj*，该包也丢失了。当客户端这边的计时器的倒计时结束了，会重新发送最后一个接受的ack消息，和重发上个未被接受的数据消息*Di*。

![](http://images2015.cnblogs.com/blog/756310/201704/756310-20170408110616441-725560684.png)


#### 1.2 LSP API

接下来就应该来写代码了。

##### 1.2.1 LSP Message
首先定义消息类型：

	//定义消息类型
	type MsgType int
	
	const (
		MsgConnect MsgType = iota  // Sent by clients to make a connection w/ the server.
		MsgData                    // Sent by clients/servers to send data.
		MsgAck                     // Sent by clients/servers to ack connect/data msgs.
	)

接下来定义消息结构体：

	//定义消息结构体
	type Message struct {
		Type    MsgType
		ConnID  int
		SeqNum  int
		Payload []byte
	}
为了从网络上发送消息，我们必须对这些数据进行序列化，然后利用UDP来发送。这一步可以使用Go语言的**Marshal**函数。

##### 1.2.2 LSP Parameters

我们需要设置LSP有关的一些参数，例如epoch最大数量，epoch超时时间，以及滑动窗口数量：

	type Params struct {
        EpochLimit  int // Default value is 5.
        EpochMillis int // Default value is 2000.
        WindowSize  int // Default value is 1.
	}
	
##### 1.2.3 LSP Client API

接下来写客户端的API。

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
	
注意：

- 读操作是阻塞的
- 写操作是非阻塞的
- 关闭操作应该是等待所有