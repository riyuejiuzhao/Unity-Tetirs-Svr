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

type IConnHandler interface {
	OnMessage(conn IConn, packet *pb.MessageWrapper)
}

type IConn interface {
	SetHandler(hander IConnHandler)
	Send(message *pb.MessageWrapper)
	Do()
}

type Conn struct {
	ctx     context.Context
	cancel  context.CancelFunc
	config  *Config
	conn    net.Conn
	handler IConnHandler

	wg          *sync.WaitGroup
	receiveChan chan *pb.MessageWrapper
	sendChan    chan *pb.MessageWrapper
}

func (c *Conn) Send(message *pb.MessageWrapper) {
	c.sendChan <- message
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
		c.conn.SetReadDeadline(time.Now().Add(c.config.receiveTimeout))
		n, err := c.conn.Read(buf)
		if err != nil {
			log.Error("读取数据失败: %v", err)
			c.cancel()
			break
		}

		message := &pb.MessageWrapper{}
		if err := proto.Unmarshal(buf[:n], message); err != nil {
			log.Error("反序列化数据失败: %v", err)
			c.cancel()
			break
		}
		c.receiveChan <- message
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
			data, err := proto.Marshal(message)
			if err != nil {
				log.Error("序列化数据失败: %v", err)
				c.cancel()
				break
			}
			c.conn.SetWriteDeadline(time.Now().Add(c.config.sendTimeout))
			_, err = c.conn.Write(data)
			if err != nil {
				log.Error("写入数据失败: %v", err)
				c.cancel()
			}
		}
	}
}

func (c *Conn) Do() {
	go c.ReceiveLoop()
	go c.SendLoop()
	go func() {
		defer func() {
			c.Close()
			c.wg.Wait()
		}()
		for {
			select {
			case <-c.ctx.Done():
				return
			case message := <-c.receiveChan:
				c.handler.OnMessage(c, message)
			}
		}
	}()

}

func NewConn(srv IServer, netConn net.Conn, handler IConnHandler) IConn {
	config := srv.Config()
	ctx, cancel := context.WithCancel(srv.Context())
	conn := &Conn{
		ctx:         ctx,
		cancel:      cancel,
		config:      config,
		conn:        netConn,
		handler:     handler,
		wg:          &sync.WaitGroup{},
		receiveChan: make(chan *pb.MessageWrapper, config.receiveChanSize),
		sendChan:    make(chan *pb.MessageWrapper, config.sendChanSize),
	}
	return conn
}
