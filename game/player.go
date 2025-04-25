package game

import "TetrisSvr/network"

type IPlayer interface {
	ID() string
	Conn() network.IConn
}

type Player struct {
	playerID string
	conn     network.IConn
}

func (p *Player) ID() string {
	return p.playerID
}

func (p *Player) Conn() network.IConn {
	return p.conn
}

// NewPlayer 创建并返回一个新的Player实例
func NewPlayer(id string, conn network.IConn) IPlayer {
	return &Player{
		playerID: id,
		conn:     conn,
	}
}
