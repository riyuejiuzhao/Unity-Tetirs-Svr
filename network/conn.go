package network

import (
	pb "TetrisSvr/proto"
	"context"
	"net"
	"sync"
	"time"

	log "github.com/jeanphorn/log4go"
	"google.golang.org/protobuf/proto"
)

type IServer interface {
	Context() context.Context
	Config() *Config
}

type ConnMessage struct {
	conn IConn
	msg  *pb.MessageWrapper
}

func (m *ConnMessage) Msg() *pb.MessageWrapper {
	return m.msg
}

func (m *ConnMessage) Conn() IConn {
	return m.conn
}

type IConnHandler interface {
	Start()
	HandleChan() chan<- *ConnMessage
}

type IConn interface {
	SetHandler(hander IConnHandler)
	SendChan() chan<- *pb.MessageWrapper
	Start()
}

type Conn struct {
	ctx     context.Context
	cancel  context.CancelFunc
	config  *Config
	conn    net.Conn
	handler IConnHandler

	wg *sync.WaitGroup
	// receiveChan chan *pb.MessageWrapper
	sendChan chan *pb.MessageWrapper
}

func (c *Conn) SendChan() chan<- *pb.MessageWrapper {
	return c.sendChan
}

func (c *Conn) SetHandler(handler IConnHandler) {
	c.handler = handler
}

func (c *Conn) Close() {
	err := c.conn.Close()
	if err != nil {
		log.Error("关闭连接失败: %v", err)
	}
}

func (c *Conn) ReceiveLoop() {
	c.wg.Add(1)
	defer c.wg.Done()

	buf := make([]byte, 1024)
	for {
		c.conn.SetReadDeadline(time.Now().Add(c.config.ReceiveTimeout))
		n, err := c.conn.Read(buf)
		if err != nil {
			log.Error("连接超时中断: %v", err)
			c.cancel()
			break
		}

		message := &pb.MessageWrapper{}
		if err := proto.Unmarshal(buf[:n], message); err != nil {
			log.Error("反序列化数据失败: %v", err)
			c.cancel()
			break
		}
		// log.Info("接收到消息: %s", message)
		c.handler.HandleChan() <- &ConnMessage{conn: c, msg: message}
	}
}

func (c *Conn) SendLoop() {
	c.wg.Add(1)
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case message := <-c.sendChan:
			log.Info("发送消息: %s", message)
			data, err := proto.Marshal(message)
			if err != nil {
				log.Error("序列化数据失败: %v", err)
				c.cancel()
				break
			}
			c.conn.SetWriteDeadline(time.Now().Add(c.config.SendTimeout))
			_, err = c.conn.Write(data)
			if err != nil {
				log.Error("写入数据失败: %v", err)
				c.cancel()
			}
		}
	}
}

func (c *Conn) Start() {
	go c.ReceiveLoop()
	go c.SendLoop()
}

func NewConn(srv IServer, netConn net.Conn, handler IConnHandler) IConn {
	config := srv.Config()
	ctx, cancel := context.WithCancel(srv.Context())

	conn := &Conn{
		ctx:      ctx,
		cancel:   cancel,
		config:   config,
		conn:     netConn,
		handler:  handler,
		wg:       &sync.WaitGroup{},
		sendChan: make(chan *pb.MessageWrapper, config.SendChanSize),
	}

	context.AfterFunc(ctx, conn.Close)
	return conn
}
