package http

import (
	"encoding/json"
	"fmt"
	"testing"
	"wireflow/internal/core/domain"
)

func TestJson(t *testing.T) {
	address := "192.168.1.101"
	address1 := "192.168.1.102"
	msg := &domain.Message{
		EventType: domain.EventTypeNodeAdd,
		Current: &domain.Peer{
			AppID:      "30a589e950",
			PrivateKey: "cOC8HdfGQsghJFPqjhEPEPNPHnoKKwyaip9ba7n/AXc=",
			Address:    &address,
		},
		Network: &domain.Network{
			Peers: []*domain.Peer{
				{
					AppID:     "30a589e950",
					PublicKey: "aaaaaaaaaaaaaaaa/AXc=",
					Address:   &address1,
				},
			},
		},
	}

	data, _ := json.Marshal(msg)
	fmt.Println(string(data))
}
