package server

import (
	"wireflow/internal/core/domain"
	"wireflow/internal/core/manager"
)

func CreateChannel(clientId string) *domain.NodeChannel {
	manager := manager.NewWatchManager()
	return manager.GetChannel(clientId)
}

func RemoveChannel(clientId string) {
	manager := manager.NewWatchManager()
	manager.Remove(clientId)
}
