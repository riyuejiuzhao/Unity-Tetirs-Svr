package network

import (
	"context"
	"net"

	log "github.com/jeanphorn/log4go"
	"github.com/xtaci/kcp-go"
)

type Server struct {
	ctx     context.Context
	config  *Config
	handler IConnHandler
}

func (m *Server) Context() context.Context {
	return m.ctx
}

func (m *Server) Config() *Config {
	// Return the server configuration
	return m.config
}

// 完成kcp 部分的启动
func (m *Server) Server(kcpAddr string) error {
	lis, err := kcp.Listen(kcpAddr)
	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := lis.Accept()
			if err == net.ErrClosed {
				break
			} else if err != nil {
				log.Error("接受连接失败: %v", err)
				continue
			}
			NewConn(m, conn, m.handler).Start()
		}
	}()

	return nil
}

func NewServer(ctx context.Context, config *Config, handler IConnHandler, kcpAddr string) Server {
	server := Server{
		ctx:     ctx,
		config:  config,
		handler: handler,
	}
	server.Server(kcpAddr)
	return server
}
