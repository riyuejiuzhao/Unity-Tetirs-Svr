package game

import (
	"TetrisSvr/network"
	"context"
	"fmt"
)

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

type IRoom interface {
	ID() string
	Status() string
	AddPlayer(playerID string, conn network.IConn) error
	RemovePlayer(playerID string) error
	Players() []IPlayer
	Game(ctx context.Context, config *network.Config) network.IConnHandler
}

type IRoomCreator interface {
	CreateRoom() (IRoom, error)
}

// 唯一ID房间创建器实现
type UniqueIDRoomCreator struct {
	nextID uint64 // 原子计数器
}

func (c *UniqueIDRoomCreator) CreateRoom() IRoom {
	c.nextID++
	return &Room{
		id:      fmt.Sprintf("%d", c.nextID),
		status:  WaitingRoom,
		players: make(map[string]IPlayer),
	}
}

type Room struct {
	id      string
	status  string
	players map[string]IPlayer
}

func (r *Room) ID() string {
	return r.id
}

func (r *Room) Status() string {
	return r.status
}

func (r *Room) AddPlayer(playerID string, conn network.IConn) error {
	if _, exists := r.players[playerID]; exists {
		return fmt.Errorf("player %s already in room", playerID)
	}
	r.players[playerID] = NewPlayer(playerID, conn)
	return nil
}

func (r *Room) RemovePlayer(playerID string) error {
	if _, exists := r.players[playerID]; !exists {
		return fmt.Errorf("player %s not found", playerID)
	}
	delete(r.players, playerID)
	return nil
}

func (r *Room) Players() []IPlayer {
	players := make([]IPlayer, 0, len(r.players))
	for _, p := range r.players {
		players = append(players, p)
	}
	return players
}

func (r *Room) Game(ctx context.Context, config *network.Config) network.IConnHandler {
	game := NewGame(ctx, r.id, config, r.players)
	return game
}
