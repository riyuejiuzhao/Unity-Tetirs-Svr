package network

import (
	"context"
	"net"
)

type IConnHandler interface {
	OnMessage(packet Packet)
}

type IConn interface {
	SetHandler(hander IConnHandler)
	Do()
}

type Conn struct {
	handler IConnHandler
}

func (c *Conn) SetHandler(handler IConnHandler) {
	c.handler = handler
}

func (c *Conn) Do() {
	// TODO
}

func NewConn(context context.Context, conn net.Conn, handler IConnHandler) IConn {
	// TODO
	return nil
}
