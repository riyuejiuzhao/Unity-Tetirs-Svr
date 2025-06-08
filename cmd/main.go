package main

import (
	"TetrisSvr/game"
	"TetrisSvr/network"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/jeanphorn/log4go"
)

func main() {
	ip := flag.String("ip", "0.0.0.0", "server listening IP")
	port := flag.Int("port", 8080, "server listening port")
	flag.Parse()

	kcpAddr := fmt.Sprintf("%s:%d", *ip, *port)

	config := &network.Config{
		ReceiveChanSize: 1024,
		ReceiveTimeout:  30 * time.Second,
		SendChanSize:    1024,
		SendTimeout:     30 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	handler := game.NewRoomManager(ctx, config, &game.UniqueIDRoomCreator{})
	handler.Start()
	server := network.NewServer(ctx, config, handler)
	server.Server(kcpAddr)

	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Info("服务器启动成功，监听地址: %s", kcpAddr)

	// 等待终止信号
	<-sigCh
	cancel()
}
