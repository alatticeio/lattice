package client

import (
	"fmt"
	"testing"
	"wireflow/pkg/log"
)

func TestClient_GetRelayInfo(t *testing.T) {
	t.Run("TestClient_GetRelayInfo", func(t *testing.T) {

		client, err := NewClient(&ClientConfig{
			ServerUrl: "stun.wireflow.io:3478",
			Logger:    log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "turnclient")),
		})

		if err != nil {
			t.Fatal(err)
		}

		info, err := client.GetRelayInfo(true)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("RelayInfo: %v", info)
		t.Log("mappped addr: ", info.MappedAddr)
	})
}
