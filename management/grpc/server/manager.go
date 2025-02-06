package server

import (
	"linkany/management/utils"
)

func CreateChannel(pubKey string) chan *utils.WatchMessage {
	manager := utils.NewWatchManager()
	ch := make(chan *utils.WatchMessage)
	manager.Add(pubKey, make(chan *utils.WatchMessage))

	return ch
}
