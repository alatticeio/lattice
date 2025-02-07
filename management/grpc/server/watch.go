package server

import "sync/atomic"

type WatchKeeper struct {
	Online atomic.Bool
}

func NewWatchKeeper() *WatchKeeper {
	return &WatchKeeper{
		Online: atomic.Bool{},
	}
}
