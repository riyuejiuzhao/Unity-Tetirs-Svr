# README

## 分析lockstepserver

对现有的框架(lockstepserver)
[https://github.com/byebyebruce/lockstepserver.git]
进行分析学习经验

### 主要过程
首先看到`pkg/kcp_server/server.go`中`ListenAndServe`
的内容，这个函数在指定端口上创建一个kcp监听。
然后它会运行`server`

`server`的`Start`会启动一个go程来等待kcp的接收到新的链接，如果有新的连接，那么给每个新的`net.Conn`包一层
`network.Conn`

`network.Conn`主要功能在`Do`这个函数上，同时启动三个go
程，分别运行`handleLoop`,`readLoop`,`writeLoop`

- readLoop: 从`protocol`中读取一个包，发送到
`packetReceiveChan`
- writeLoop：从`packetSendChan`中读取一个包，然后写入
`conn`中
- handleLoop：从`packetReceiveChan`中读取一个包，然后根
据`network.conn`的`callback`进行处理，这里注意，
`network.conn`的`callback`是和`server`的`callback`相同
的

设置`callback`和`protocol`的语句为
```go
server := network.NewServer(config, callback, &network.DefaultProtocol{})
```
这两个参数都可以根据用户的需求客制化，实际上不是框架内的。

先看一下`callback`
```go
type ConnCallback interface {
	// OnConnect is called when the connection was accepted,
	// If the return value of false is closed
	OnConnect(*Conn) bool

	// OnMessage is called when the connection receives a packet,
	// If the return value of false is closed
	OnMessage(*Conn, Packet) bool

	// OnClose is called when the connection closed
	OnClose(*Conn)
}
```
它说明了每个`message`到达后应该如何处理，`LockStepServer`
就是一个`ConnCallback`

`Protocol`的功能只有一个，那就是读取`Packet`，而Packet
主要功能就是序列化
```go
type Protocol interface {
	ReadPacket(conn io.Reader) (Packet, error)
}
```

这里我们注意一点，当LockStepServer接收到游戏开始的信号后，它会
对玩家身份进行验证然后将`Callback`交给`Room`类，可以参照
```go
// OnConnect network.Conn callback
func (r *Room) OnConnect(conn *network.Conn) bool {

	conn.SetCallback(r) // SetCallback只能在OnConnect里调
	r.inChan <- conn
	l4g.Warn("[room(%d)] OnConnect %d", r.roomID, conn.GetExtraData().(uint64))

	return true
}
```

### RoomManager
房间管理类，主要功能是创建房间，删除房间

### Room
房间类，实际上起到管理游戏的作用。

房间类接收到的信息最终通过`game.ProcessMsg`对报文进行处理。

我们的帧数据就是接受用户输入报文`C2S_InputMsg`

需要注意的是，游戏内有部分报文，主要是帧数据，是game的`Tick`在处理

有个地方不太理解，为什么InputData不包含这个Input
输入的FrameNumber呢？

## 我的实现

服务器工作流程
- 启动服务器，监听kcp报文
- 收到创建房间报文，创建房间并且加入对应的玩家
- 将对应报文的处理权转让给对应房间结构体
- 等待新的连接和房间的创建

### 房间

房间工作流程：
- 初始状态等待玩家数量足够，暂定为两人
- 玩家数量足够后，任意玩家可以开始游戏
- 在收到开始游戏的报文后，向两个玩家发送加载报文，同时本地创建游戏
- 在两个玩家加载完成，并且本地游戏创建完成后，将报文处理权交给游戏体

房间状态：
- 等待玩家：此时玩家数量上还不足一开始游戏
- 准备就绪：此时玩家数量已经满足条件，可以开始游戏
- 等待加载：等待两个玩家把游戏加载完毕
- 运行游戏：

