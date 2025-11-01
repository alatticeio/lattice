package http

import (
	"encoding/json"
	"fmt"
	"testing"
	"wireflow/internal"
)

func TestJson(t *testing.T) {
	msg := &internal.Message{
		EventType: internal.EventTypeNodeAdd,
		Current: &internal.Node{
			AppID:      "30a589e950",
			PrivateKey: "cOC8HdfGQsghJFPqjhEPEPNPHnoKKwyaip9ba7n/AXc=",
			Address:    "192.168.1.101",
		},
		Network: &internal.Network{
			Nodes: []*internal.Node{
				{
					AppID:     "30a589e950",
					PublicKey: "aaaaaaaaaaaaaaaa/AXc=",
					Address:   "192.168.1.102",
				},
			},
		},
	}

	data, _ := json.Marshal(msg)
	fmt.Println(string(data))
}
