package server

import (
	"linkany/internal"
)

func CreateChannel(pubKey string) *internal.NodeChannel {
	manager := internal.NewWatchManager()
	return manager.GetChannel(pubKey)
}

func RemoveChannel(pubKey string) {
	manager := internal.NewWatchManager()
	manager.Remove(pubKey)
}
