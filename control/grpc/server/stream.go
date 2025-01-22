package server

import (
	pb "linkany/control/grpc/peer"
	"sync"
)

// StreamManager is a struct that manages the stream of watch responses, every user will has its own userStreamManager
type StreamManager struct {
	lock    sync.Mutex
	manager map[string]*userStreamManager // key is userId
}

func NewStreamManager() *StreamManager {
	return &StreamManager{
		manager: make(map[string]*userStreamManager),
	}
}

func (s *StreamManager) AddStream(userId string, appId string, stream pb.ListWatcher_WatchServer) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.manager[userId]; !ok {
		s.manager[userId] = newUserStreamManager()
	}

	s.manager[userId].addStream(appId, stream)
}

func (s *StreamManager) RemoveStream(userId string, appId string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.manager[userId]; ok {
		s.manager[userId].removeStream(appId)
	}
}

func (s *StreamManager) GetStream(userId string, appId string) *Stream {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.manager[userId]; ok {
		return s.manager[userId].getStream(appId)
	}

	return nil
}
