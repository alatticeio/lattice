package server

import (
	"fmt"
	"linkany/internal"
	"testing"
	"time"
)

func TestCreateChannel(t *testing.T) {
	pubKey := "123456"
	ch := CreateChannel(pubKey)
	//ch := make(chan *mgt.HandleWatchMessage, 1000)
	manager := internal.NewWatchManager()
	//manager.Add(pubKey, ch)

	go func() {
		for {
			select {
			case c := <-ch.GetChannel():
				fmt.Println("got message", c)
			}
		}
	}()

	go func() {
		//manager := utils.NewWatchManager()
		manager.Push(pubKey, &internal.Message{})
	}()

	time.Sleep(1000 * time.Second)
}
