package server

import (
	"linkany/management/vo"
)

func CreateChannel(pubKey string) chan *vo.Message {
	manager := vo.NewWatchManager()
	ch := make(chan *vo.Message, 1000)
	manager.Add(pubKey, ch)

	return ch
}

func RemoveChannel(pubKey string) {
	manager := vo.NewWatchManager()
	manager.Remove(pubKey)
}
