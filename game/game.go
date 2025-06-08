package game

import (
	"TetrisSvr/network"
	pb "TetrisSvr/proto"
	"context"
	"time"

	log "github.com/jeanphorn/log4go"
)

const WaitingGame = "waiting"
const PlayingGame = "playing"
const GameOver = "gameover"

type FrameData struct {
	Operations [][]byte
}

type GamePlayer struct {
	playerID string
	conn     network.IConn
	frames   map[int32]*FrameData
	complete *pb.C2S_GameLoadComplete

	ready           bool
	lastFrameNumber int32 // 玩家最后确认的帧号
	lastSentFrame   int32 // 最后成功发送的帧号
	ended           bool  // 玩家是否已结束游戏
}

func (p *GamePlayer) AddInput(frame int32, op []byte) {
	if p.frames == nil {
		p.frames = make(map[int32]*FrameData)
	}

	if _, exists := p.frames[frame]; !exists {
		p.frames[frame] = &FrameData{
			Operations: make([][]byte, 0),
		}
	}
	p.frames[frame].Operations = append(p.frames[frame].Operations, op)
}

// Game 实现IRoom接口的俄罗斯方块游戏房间
type Game struct {
	gameID      string
	context     context.Context
	status      string
	players     map[string]*GamePlayer
	messageChan chan *network.ConnMessage

	ticker      *time.Ticker
	frameNumber int32
}

// NewGame 创建新的游戏实例
func NewGame(ctx context.Context, gameID string, config *network.Config, players map[string]IPlayer) *Game {
	gamePlayers := make(map[string]*GamePlayer)
	for _, player := range players {
		gamePlayers[player.ID()] = &GamePlayer{
			playerID:        player.ID(),
			conn:            player.Conn(),
			frames:          make(map[int32]*FrameData),
			ready:           false,
			lastFrameNumber: -1,
		}
	}
	return &Game{
		gameID:      gameID,
		context:     ctx,
		status:      WaitingGame,
		players:     gamePlayers,
		messageChan: make(chan *network.ConnMessage, config.ReceiveChanSize),
		frameNumber: 0,
	}
}

// 实现IRoom接口
func (g *Game) ID() string {
	return g.gameID
}

func (g *Game) Start() {
	go g.gameLoop()
}

func (g *Game) HandleChan() chan<- *network.ConnMessage {
	return g.messageChan
}

func (g *Game) handleGameLoadComplete(message *pb.C2S_GameLoadComplete) {
	playerID := message.GetPlayerId()
	g.players[playerID].ready = true
	g.players[playerID].complete = message

	log.Info("Player %s is ready", playerID)
	for _, p := range g.players {
		if !p.ready {
			return
		}
	}

	reply := &pb.S2C_GameLoadComplete{
		Msg: make([]*pb.C2S_GameLoadComplete, 0, len(g.players)),
	}
	for _, p := range g.players {
		reply.Msg = append(reply.Msg, p.complete)
	}
	for _, p := range g.players {
		p.conn.SendChan() <- &pb.MessageWrapper{
			Msg: &pb.MessageWrapper_S2CGameLoadComplete{
				S2CGameLoadComplete: reply,
			},
		}
	}
	g.status = PlayingGame
	g.ticker = time.NewTicker(time.Second / 30)
	g.frameNumber = 0
	log.Info("All players are ready, game started")
}

func (g *Game) handleWaitingMessage(_ network.IConn, message *pb.MessageWrapper) {
	switch msg := message.Msg.(type) {
	case *pb.MessageWrapper_C2SGameLoadComplete:
		g.handleGameLoadComplete(msg.C2SGameLoadComplete)
	case *pb.MessageWrapper_C2SHeartbeat:
		// log.Info("Heartbeat:%s received", msg.C2SHeartbeat.GetPlayerId())
		return
	default:
		log.Error("Unknown message type: %T", msg)
	}
}

func (g *Game) handleInput(_ network.IConn, message *pb.C2S_Input) {
	playerID := message.GetPlayerId()
	currentFrame := g.frameNumber

	if player, ok := g.players[playerID]; ok {
		player.AddInput(currentFrame, message.GetOperations())
		log.Debug("Added %d operations to frame %d for player %s",
			len(message.GetOperations()), currentFrame, playerID)
	} else {
		log.Error("Player %s not found when handling input", playerID)
	}
}

func (g *Game) handleGameEnd(_ network.IConn, message *pb.C2S_GameEnd) {
	playerID := message.GetPlayerId()
	endRequest := message.GetEndGame()

	// 构造广播消息
	broadcastMsg := &pb.MessageWrapper{
		Msg: &pb.MessageWrapper_S2CGameEnd{
			S2CGameEnd: &pb.S2C_GameEnd{
				EndPlayer: playerID,
				EndGame:   endRequest,
				Payload:   message.GetPayload(),
			},
		},
	}

	// 广播给所有玩家
	for _, p := range g.players {
		select {
		case p.conn.SendChan() <- broadcastMsg:
		default:
			log.Warn("Failed to send game end message to %s", p.playerID)
		}
	}

	if endRequest {
		// 强制结束游戏
		g.endGame()
		return
	}

	// 标记当前玩家已结束
	if player, exists := g.players[playerID]; exists {
		player.ended = true
		log.Info("Player %s has ended the game", playerID)
	}

	// 检查是否所有玩家都已结束
	if g.allPlayersEnded() {
		log.Info("All players have ended, terminating game")
		g.endGame()
	}
}

func (g *Game) allPlayersEnded() bool {
	for _, p := range g.players {
		if !p.ended {
			return false
		}
	}
	return true
}

func (g *Game) endGame() {
	g.status = GameOver
	if g.ticker != nil {
		g.ticker.Stop()
	}
	// 安全关闭消息通道
	if g.messageChan != nil {
		close(g.messageChan)
		g.messageChan = nil
	}
	log.Info("Game %s ended", g.gameID)
}

func (g *Game) handlePlayingMessage(conn network.IConn, message *pb.MessageWrapper) {
	switch msg := message.Msg.(type) {
	case *pb.MessageWrapper_C2SInput:
		g.handleInput(conn, msg.C2SInput)
		return
	case *pb.MessageWrapper_C2SHeartbeat:
		// log.Info("Heartbeat:%s received", msg.C2SHeartbeat.GetPlayerId())
		return
	case *pb.MessageWrapper_C2SGameEnd:
		g.handleGameEnd(conn, msg.C2SGameEnd)
		return
	default:
		log.Error("Unknown message type: %T", msg)
	}
}

func (g *Game) tick() {
	// 给每个接收玩家处理
	for _, receiver := range g.players {
		start := receiver.lastSentFrame
		end := g.frameNumber
		if start > end {
			continue
		}

		// 按玩家ID组织帧数据
		playerFrames := make([]*pb.S2C_PlayerFrames, 0, len(g.players))
		// 收集所有玩家的帧数据（包括自己）
		for _, player := range g.players {
			var frames []*pb.S2C_Frame
			for frameNum := start; frameNum <= end; frameNum++ {
				frame := &pb.S2C_Frame{
					FrameNumber: frameNum,
				}
				if data, exists := player.frames[frameNum]; exists {
					frame.Operations = data.Operations
				}
				frames = append(frames, frame)
			}
			playerFrames = append(playerFrames, &pb.S2C_PlayerFrames{
				PlayerId: player.playerID,
				Frames:   frames,
			})
		}

		syncMsg := &pb.MessageWrapper{
			Msg: &pb.MessageWrapper_S2CSyncFrames{
				S2CSyncFrames: &pb.S2C_SyncFrames{
					Players: playerFrames,
				},
			},
		}

		select {
		case receiver.conn.SendChan() <- syncMsg:
			receiver.lastSentFrame = end
			// log.Info("Sent %d player frames to %s", len(playerFrames), receiver.playerID)
		default:
			log.Warn("Failed to send %d player frames to %s", len(playerFrames), receiver.playerID)
		}
	}
	g.frameNumber++
}

// 游戏主循环
func (g *Game) gameLoop() {
	log.Info("Game %s started", g.gameID)
	for {
		switch g.status {
		case WaitingGame:
			select {
			case <-g.context.Done():
				return
			case msg := <-g.messageChan:
				g.handleWaitingMessage(msg.Conn(), msg.Msg())
			}
		case PlayingGame:
			select {
			case <-g.context.Done():
				return
			case msg := <-g.messageChan:
				g.handlePlayingMessage(msg.Conn(), msg.Msg())
			case <-g.ticker.C:
				g.tick()
			}
		case GameOver:
			select {
			case <-g.context.Done():
				return
			case <-g.messageChan:
				log.Error("Game over, no more messages will be processed")
			}
		}
	}
}
