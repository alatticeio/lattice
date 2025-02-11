package client

import "testing"

func TestClient_GetRelayInfo(t *testing.T) {
	t.Run("TestClient_GetRelayInfo", func(t *testing.T) {

		client, err := NewClient(&ClientConfig{
			ServerUrl: "stun.linkany.io:3478",
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
