package main

import (
	pb "TetrisSvr/proto"
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type PlayerInfo struct {
	id           int32
	initInfo     *pb.ClientInit
	toInitChan   map[int32]chan<- *pb.ClientInit
	fromInitChan map[int32]<-chan *pb.ClientInit

	frames []*pb.FrameUpdate

	selfChan      chan *pb.FrameUpdate
	toOtherChan   map[int32]chan<- *pb.FrameUpdate
	fromOtherChan map[int32]<-chan *pb.FrameUpdate
}

func NewPlayerInfo(id int32) *PlayerInfo {
	return &PlayerInfo{
		id:           id,
		initInfo:     nil,
		toInitChan:   make(map[int32]chan<- *pb.ClientInit),
		fromInitChan: make(map[int32]<-chan *pb.ClientInit),

		frames: make([]*pb.FrameUpdate, 0),

		selfChan:      make(chan *pb.FrameUpdate, 1000),
		toOtherChan:   make(map[int32]chan<- *pb.FrameUpdate),
		fromOtherChan: make(map[int32]<-chan *pb.FrameUpdate),
	}
}

func (p *PlayerInfo) SetInit(init *pb.ClientInit) {
	p.initInfo = init
	for _, c := range p.toInitChan {
		c <- init
	}
}

func (p *PlayerInfo) AddFrame(frame *pb.FrameUpdate) {
	p.frames = append(p.frames, frame)
	p.selfChan <- frame
	for _, c := range p.toOtherChan {
		c <- frame
	}
}

type Room struct {
	players map[int32]*PlayerInfo
}

func NewRoom(players []int32) *Room {
	playersInfo := make(map[int32]*PlayerInfo)
	for _, id := range players {
		playersInfo[id] = NewPlayerInfo(id)
	}
	return &Room{players: playersInfo}
}

// 定义 TetrisService 服务
type TetrisService struct {
	pb.UnimplementedTetrisServiceServer
	matchPool map[int32]chan *pb.MatchSuccess // 匹配池（玩家ID -> 匹配成功通知通道）
	play2room map[int32]*Room

	mu sync.Mutex
}

func (s *TetrisService) Ping(context.Context, *pb.Empty) (*pb.Empty, error) {
	fmt.Println("ping")
	return &pb.Empty{}, nil
}

// 尝试匹配
func (s *TetrisService) tryMatch(playID int32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.matchPool) >= 2 {
		// 找到两名玩家
		players := []int32{playID}
		for id := range s.matchPool {
			if id == playID {
				continue
			}
			players = append(players, id)
			break
		}

		// 创建战斗房间
		room := NewRoom(players)
		for _, id := range players {
			fromInfo := room.players[id]
			s.play2room[id] = room
			for _, opponentID := range players {
				if opponentID == id {
					continue
				}
				toInfo := room.players[opponentID]
				s.matchPool[id] <- &pb.MatchSuccess{
					OpponentId: opponentID,
				}

				frameChan := make(chan *pb.FrameUpdate, 1000)
				initChan := make(chan *pb.ClientInit, 1)

				toInfo.fromOtherChan[id] = frameChan
				toInfo.fromInitChan[id] = initChan
				fromInfo.toOtherChan[opponentID] = frameChan
				fromInfo.toInitChan[opponentID] = initChan
			}
			delete(s.matchPool, id)
		}
	}
}

func (s *TetrisService) WaitForMatch(req *pb.WaitForMatchRequest, stream pb.TetrisService_WaitForMatchServer) error {
	playerID := req.GetPlayerId()
	s.mu.Lock()
	ch, exists := s.matchPool[playerID]
	if !exists {
		// 如果玩家不在匹配池中，创建一个新的通道
		ch = make(chan *pb.MatchSuccess, 1)
		s.matchPool[playerID] = ch
	}
	s.mu.Unlock()

	// 尝试匹配
	s.tryMatch(playerID)

	// 挂起客户端，直到匹配成功
	matchResult := <-ch
	return stream.Send(matchResult)
}

func (s *TetrisService) InitGame(_ context.Context, req *pb.ClientInit) (*pb.Empty, error) {
	slog.Info(fmt.Sprintf("InitGame %v", req))
	// 记录客户端初始化信息
	s.mu.Lock()
	defer s.mu.Unlock()

	room := s.play2room[req.PlayerId]
	if room.players[req.PlayerId] == nil {
		room.players[req.PlayerId] = NewPlayerInfo(req.PlayerId)
	}
	room.players[req.PlayerId].SetInit(req)
	return &pb.Empty{}, nil
}

func (s *TetrisService) SyncInit(_ context.Context, req *pb.SyncInitRequest) (*pb.SyncInitReply, error) {
	slog.Info(fmt.Sprintf("SyncInit %v", req))

	s.mu.Lock()
	room := s.play2room[req.PlayerId]
	playerInfo := room.players[req.PlayerId].initInfo
	otherChan := room.players[req.PlayerId].fromInitChan
	s.mu.Unlock()

	clients := make([]*pb.ClientInit, 0)
	clients = append(clients, playerInfo)

	for _, c := range otherChan {
		clients = append(clients, <-c)
	}
	return &pb.SyncInitReply{Clients: clients}, nil
}

func (s *TetrisService) SendFrame(_ context.Context, frameUpdate *pb.FrameUpdate) (*pb.Empty, error) {
	slog.Info(fmt.Sprintf("SendFrame %v", frameUpdate))
	s.mu.Lock()
	defer s.mu.Unlock()

	playerInfo := s.play2room[frameUpdate.PlayerId].players[frameUpdate.PlayerId]
	playerInfo.AddFrame(frameUpdate)

	return &pb.Empty{}, nil
}

func (s *TetrisService) SyncFrame(req *pb.SyncFrameRequest, stream pb.TetrisService_SyncFrameServer) error {
	slog.Info(fmt.Sprintf("SyncFrame %v", req))

	s.mu.Lock()
	room := s.play2room[req.PlayerId]
	selfChan := room.players[req.PlayerId].selfChan
	otherChan := room.players[req.PlayerId].fromOtherChan
	s.mu.Unlock()

	for {
		frames := make([]*pb.FrameUpdate, 0)
		frames = append(frames, <-selfChan)
		for _, c := range otherChan {
			frames = append(frames, <-c)
		}
		reply := &pb.SyncFrameReply{Frames: frames}
		stream.Send(reply)
		slog.Info(fmt.Sprintf("SyncFrame %v", reply))
	}
}

func main() {
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	server := grpc.NewServer()
	reflection.Register(server)
	service := &TetrisService{
		matchPool: make(map[int32]chan *pb.MatchSuccess), // 匹配池（玩家ID -> 匹配成功通知通道）
		play2room: make(map[int32]*Room),
		mu:        sync.Mutex{},
	}
	pb.RegisterTetrisServiceServer(server, service)

	log.Println("Server started. Listening...")
	if err := server.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
