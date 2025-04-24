package room

import "TetrisSvr/network"

type RoomManager struct {
}

func (m *RoomManager) OnMessage(packet network.Packet) bool {
	return true
}
