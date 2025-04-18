package server

import (
	"linkany/management/utils"
)

func CreateChannel(pubKey string) *utils.NodeChannel {
	manager := utils.NewWatchManager()
	return manager.GetChannel(pubKey)
}

func RemoveChannel(pubKey string) {
	manager := utils.NewWatchManager()
	manager.Remove(pubKey)
}
