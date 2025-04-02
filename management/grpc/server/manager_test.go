package server

import (
	"fmt"
	"linkany/management/grpc/mgt"
	"linkany/management/utils"
	"testing"
	"time"
)

func TestCreateChannel(t *testing.T) {
	pubKey := "123456"
	ch := CreateChannel(pubKey)
	//ch := make(chan *mgt.HandleWatchMessage, 1000)
	manager := utils.NewWatchManager()
	//manager.Add(pubKey, ch)

	go func() {
		for {
			select {
			case c := <-ch:
				fmt.Println("got message", c)
			}
		}
	}()

	go func() {
		//manager := utils.NewWatchManager()
		ch := manager.Get(pubKey)
		for i := 0; i < 10; i++ {
			ch <- &mgt.WatchMessage{
				Type: mgt.EventType_ADD,
				Peer: &mgt.Peer{
					PublicKey: pubKey,
				},
			}
		}
	}()

	time.Sleep(1000 * time.Second)
}
