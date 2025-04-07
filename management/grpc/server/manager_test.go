package server

import (
	"fmt"
	"linkany/management/vo"
	"testing"
	"time"
)

func TestCreateChannel(t *testing.T) {
	pubKey := "123456"
	ch := CreateChannel(pubKey)
	//ch := make(chan *mgt.HandleWatchMessage, 1000)
	manager := vo.NewWatchManager()
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
			ch <- &vo.Message{}
		}
	}()

	time.Sleep(1000 * time.Second)
}
