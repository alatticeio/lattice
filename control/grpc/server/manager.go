package server

import (
	pb "linkany/control/grpc/peer"
	"sync"
)

// userStreamManager is a struct that manages the stream of watch responses
// m is a map of string to channel of watch response, key is the appId of the peer, every peer has its own appId
type userStreamManager struct {
	userId string
	lock   sync.Mutex
	m      map[string]*Stream
}

type Stream struct {
	userId    string
	appId     string
	stream    pb.ListWatcher_WatchServer
	dataQueue chan *pb.WatchResponse
}

type StreamConfig struct {
	UserId    string
	AppId     string
	Stream    pb.ListWatcher_WatchServer
	DataQueue chan *pb.WatchResponse
}

func newStream(cfg *StreamConfig) *Stream {
	return &Stream{
		userId:    cfg.UserId,
		appId:     cfg.AppId,
		stream:    cfg.Stream,
		dataQueue: cfg.DataQueue,
	}
}

func newUserStreamManager() *userStreamManager {
	return &userStreamManager{
		m: make(map[string]*Stream),
	}
}

func (s *userStreamManager) addStream(appId string, stream pb.ListWatcher_WatchServer) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.m[appId] = newStream(&StreamConfig{
		AppId:     appId,
		Stream:    stream,
		DataQueue: make(chan *pb.WatchResponse, 100),
	})
}

func (s *userStreamManager) removeStream(appId string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.m, appId)
}

func (s *userStreamManager) getStream(appId string) *Stream {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.m[appId]
}
