package room

import (
	"TetrisSvr/network"
	pb "TetrisSvr/proto"

	log "github.com/jeanphorn/log4go"
)

type IRoom interface {
	ID() string
	AddPlayer(playerID string) error
	RemovePlayer(playerID string) error
	PlayerExists(playerID string) bool
}

type IRoomCreator interface {
	CreateRoom() (IRoom, error)
}

type RoomManager struct {
	creator IRoomCreator
	rooms   map[string]IRoom
}

func (m *RoomManager) handleEnterRoom(conn network.IConn, message *pb.C2S_EnterRoom) {
	replyMsg := &pb.S2C_EnterRoom{}

	defer func() {
		reply := &pb.MessageWrapper{
			Msg: &pb.MessageWrapper_S2CEnterRoom{
				S2CEnterRoom: replyMsg,
			},
		}
		conn.Send(reply)
	}()

	roomID := message.GetRoomId()
	playerID := message.GetPlayerId()
	r, ok := m.rooms[roomID]
	if !ok {
		log.Error("Room not found: %s", roomID)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Room not found"
		return
	}
	err := r.AddPlayer(playerID)
	if err != nil {
		log.Error("Failed to add player to room: %v", err)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Failed to add player to room"
		return
	}
	replyMsg.Error = false
}

func (m *RoomManager) handleCreateRoom(conn network.IConn, message *pb.C2S_CreateRoom) {
	replyMsg := &pb.S2C_CreateRoom{}

	defer func() {
		reply := &pb.MessageWrapper{
			Msg: &pb.MessageWrapper_S2CCreateRoom{
				S2CCreateRoom: replyMsg,
			},
		}
		conn.Send(reply)
	}()

	playerID := message.GetPlayerId()
	r, err := m.creator.CreateRoom()
	if err != nil {
		log.Error("Failed to create room: %v", err)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Failed to create room"
		return
	}
	m.rooms[r.ID()] = r
	err = r.AddPlayer(playerID)
	if err != nil {
		log.Error("Failed to add player to room: %v", err)
		replyMsg.Error = true
		replyMsg.ErrorMsg = "Failed to add player to room"
		return
	}

	replyMsg.RoomId = r.ID()
	replyMsg.PlayerId = playerID
	replyMsg.Error = false
}

func (m *RoomManager) handleExitRoom(conn network.IConn, message *pb.C2S_ExitRoom) {
	replyMsg := &pb.S2C_ExitRoom{}

	defer func() {
		reply := &pb.MessageWrapper{
			Msg: &pb.MessageWrapper_S2CExitRoom{
				S2CExitRoom: replyMsg,
			},
		}
		conn.Send(reply)
	}()

	roomID := message.GetRoomId()
	playerID := message.GetPlayerId()
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

	replyMsg.Error = false
}

func (m *RoomManager) OnMessage(conn network.IConn, packet *pb.MessageWrapper) bool {
	message := packet.Msg
	switch payload := message.(type) {
	case *pb.MessageWrapper_C2SEnterRoom:
		m.handleEnterRoom(conn, payload.C2SEnterRoom)
	case *pb.MessageWrapper_C2SCreateRoom:
		m.handleCreateRoom(conn, payload.C2SCreateRoom)
	case *pb.MessageWrapper_C2SExitRoom:
		m.handleExitRoom(conn, payload.C2SExitRoom)
	default:
		log.Error("Unknown message type: %T", payload)
	}

	return true
}

func NewRoomManager(creator IRoomCreator) *RoomManager {
	return &RoomManager{
		creator: creator,
		rooms:   make(map[string]IRoom),
	}
}
