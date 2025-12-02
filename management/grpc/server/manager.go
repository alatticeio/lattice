package server

import (
	"wireflow/internal"
)

func CreateChannel(clientId string) *internal.NodeChannel {
	manager := internal.NewWatchManager()
	return manager.GetChannel(clientId)
}

func RemoveChannel(clientId string) {
	manager := internal.NewWatchManager()
	manager.Remove(clientId)
}
