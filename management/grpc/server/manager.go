package server

import (
	"linkany/management/grpc/mgt"
	"linkany/management/utils"
)

func CreateChannel(pubKey string) chan *mgt.WatchMessage {
	manager := utils.NewWatchManager()
	ch := make(chan *mgt.WatchMessage, 1000)
	manager.Add(pubKey, ch)

	return ch
}

func RemoveChannel(pubKey string) {
	manager := utils.NewWatchManager()
	manager.Remove(pubKey)
}
