package srv

import (
	"TetrisSvr/network"
	"context"
	"net"

	log "github.com/jeanphorn/log4go"
	"github.com/xtaci/kcp-go"
)

type Server struct {
	ctx     context.Context
	handler network.IConnHandler
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
			network.NewConn(m.ctx, conn, m.handler).Do()
		}
	}()

	return nil
}

func NewServer(ctx context.Context, handler network.IConnHandler, kcpAddr string) Server {
	server := Server{
		ctx:     ctx,
		handler: handler,
	}
	server.Server(kcpAddr)
	return server
}
