package game

import (
	"TetrisSvr/network"
	pb "TetrisSvr/proto"
	"context"

	log "github.com/jeanphorn/log4go"
)

const WaitingRoom = "waiting_room"
const GameRoom = "game_room"

type RoomManager struct {
	ctx         context.Context
	cfg         *network.Config
	creator     IRoomCreator
	rooms       map[string]IRoom
	player2room map[string]IRoom
	handleChan  chan *network.ConnMessage
}

func (m *RoomManager) HandleChan() chan<- *network.ConnMessage {
	return m.handleChan
}

func (m *RoomManager) broadcastRoomInfoChanged(roomID string, playerID string) {
	room := m.rooms[roomID]
	players := room.Players()
	playerIDs := make([]string, len(players))
	for i, p := range players {
		playerIDs[i] = p.ID()
	}
	reply := &pb.MessageWrapper{
		Msg: &pb.MessageWrapper_S2CRoomInfoChanged{
			S2CRoomInfoChanged: &pb.S2C_RoomInfoChanged{
				RoomId:    roomID,
				PlayerIds: playerIDs,
			},
		},
	}
	for _, p := range players {
		if p.ID() == playerID {
			// Skip sending to the player who triggered the change
			continue
		}
		p.Conn().SendChan() <- reply
	}
}

func (m *RoomManager) handleEnterRoom(conn network.IConn, message *pb.C2S_EnterRoom) {
	// log.Info("接收到消息: %s", message)
	replyMsg := &pb.S2C_EnterRoom{}
	roomID := message.GetRoomId()
	playerID := message.GetPlayerId()

	defer func() {
		reply := &pb.MessageWrapper{
			Msg: &pb.MessageWrapper_S2CEnterRoom{
				S2CEnterRoom: replyMsg,
			},
		}
		conn.SendChan() <- reply
		if !replyMsg.Error {
			m.broadcastRoomInfoChanged(roomID, playerID)
		}
	}()

	r, ok := m.rooms[roomID]
	if !ok {
		log.Error("Room not found: %s", roomID)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Room not found"
		return
	}
	if r.Status() == GameRoom {
		log.Error("Room is already in game: %s", roomID)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Room is already in game"
		return
	}
	err := r.AddPlayer(playerID, conn)
	if err != nil {
		log.Error("Failed to add player to room: %v", err)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Failed to add player to room"
		return
	}
	m.player2room[playerID] = r
	replyMsg.Error = false

	players := r.Players()
	playerIDs := make([]string, len(players))
	for i, p := range players {
		playerIDs[i] = p.ID()
	}

	replyMsg.Info = &pb.S2C_RoomInfoChanged{
		RoomId:    roomID,
		PlayerIds: playerIDs,
	}
}

func (m *RoomManager) handleCreateRoom(conn network.IConn, message *pb.C2S_CreateRoom) {
	log.Info("接收到消息: %s", message)
	replyMsg := &pb.S2C_CreateRoom{}

	defer func() {
		reply := &pb.MessageWrapper{
			Msg: &pb.MessageWrapper_S2CCreateRoom{
				S2CCreateRoom: replyMsg,
			},
		}
		log.Info("Sending reply: %v", reply)
		conn.SendChan() <- reply
	}()

	playerID := message.GetPlayerId()
	_, ok := m.player2room[playerID]
	if ok {
		log.Error("Player already in a room: %s", playerID)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Player already in a room"
		return
	}

	r, err := m.creator.CreateRoom()
	if err != nil {
		log.Error("Failed to create room: %v", err)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Failed to create room"
		return
	}
	m.rooms[r.ID()] = r
	err = r.AddPlayer(playerID, conn)
	if err != nil {
		log.Error("Failed to add player to room: %v", err)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Failed to add player to room"
		return
	}
	m.player2room[playerID] = r

	replyMsg.Info = &pb.S2C_RoomInfoChanged{
		RoomId:    r.ID(),
		PlayerIds: []string{playerID},
	}
	replyMsg.Error = false
}

func (m *RoomManager) handleStartGame(conn network.IConn, message *pb.C2S_StartGame) {
	log.Info("接收到消息: %s", message)
	replyMsg := &pb.S2C_StartGame{}
	roomID := message.GetRoomId()

	defer func() {
		reply := &pb.MessageWrapper{
			Msg: &pb.MessageWrapper_S2CStartGame{
				S2CStartGame: replyMsg,
			},
		}
		// Send to all players on success, only requester on error
		if !replyMsg.Error {
			room := m.rooms[roomID]
			handler := room.Game(m.ctx, m.cfg)
			handler.Start()
			for _, p := range room.Players() {
				p.Conn().SetHandler(handler)
				p.Conn().SendChan() <- reply
			}
		} else {
			conn.SendChan() <- reply
		}
	}()

	_, ok := m.rooms[roomID]
	if !ok {
		log.Error("Room not found: %s", roomID)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Room not found"
		return
	}

	// Validate game start conditions
	// players := r.Players()
	// if len(players) < 2 {
	// 	replyMsg.Error = true
	// 	replyMsg.ErrorMsg = "Need at least 2 players to start"
	// 	return
	// }

	replyMsg.Error = false
}

func (m *RoomManager) handleExitRoom(conn network.IConn, message *pb.C2S_ExitRoom) {
	log.Info("接收到消息: %s", message)
	replyMsg := &pb.S2C_ExitRoom{}
	roomID := message.GetRoomId()
	playerID := message.GetPlayerId()

	defer func() {
		if !replyMsg.Error {
			m.broadcastRoomInfoChanged(roomID, playerID)
		}
		reply := &pb.MessageWrapper{
			Msg: &pb.MessageWrapper_S2CExitRoom{
				S2CExitRoom: replyMsg,
			},
		}
		conn.SendChan() <- reply
	}()

	r, ok := m.rooms[roomID]
	if !ok {
		log.Error("Room not found: %s", roomID)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Room not found"
		return
	}
	err := r.RemovePlayer(playerID)
	if err != nil {
		log.Error("Failed to remove player from room: %v", err)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Failed to remove player from room"
		return
	}
	delete(m.player2room, playerID)

	replyMsg.Error = false
}

func (m *RoomManager) handleMessage(conn network.IConn, packet *pb.MessageWrapper) bool {
	message := packet.Msg
	// log.Info("Received message: %T", message)
	switch payload := message.(type) {
	case *pb.MessageWrapper_C2SEnterRoom:
		m.handleEnterRoom(conn, payload.C2SEnterRoom)
	case *pb.MessageWrapper_C2SCreateRoom:
		m.handleCreateRoom(conn, payload.C2SCreateRoom)
	case *pb.MessageWrapper_C2SExitRoom:
		m.handleExitRoom(conn, payload.C2SExitRoom)
	case *pb.MessageWrapper_C2SStartGame:
		m.handleStartGame(conn, payload.C2SStartGame)
	case *pb.MessageWrapper_C2SHeartbeat:
		// Heartbeat message, no action needed
	default:
		log.Error("Unknown message type: %s", payload)
	}

	return true
}

func (m *RoomManager) Start() {
	// log.Info("room mgr start")
	go func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case msg := <-m.handleChan:
				m.handleMessage(msg.Conn(), msg.Msg())
			}

		}
	}()
}

func NewRoomManager(context context.Context, config *network.Config, creator IRoomCreator) *RoomManager {
	return &RoomManager{
		ctx:         context,
		cfg:         config,
		creator:     creator,
		rooms:       make(map[string]IRoom),
		handleChan:  make(chan *network.ConnMessage, config.ReceiveChanSize),
		player2room: make(map[string]IRoom),
	}
}
