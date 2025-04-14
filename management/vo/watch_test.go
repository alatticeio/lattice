package vo

import (
	"encoding/json"
	"testing"
)

func TestNewMessage(t *testing.T) {
	// a node event

	message := NewMessage(&MessageConfig{
		EventType: EventTypeNodeAdd,
		GroupMessage: &GroupMessage{
			Nodes: []*NodeVo{
				&NodeVo{
					ID:      1,
					Name:    "test",
					Address: "192.168.0.3",
				},
				&NodeVo{
					ID:      2,
					Name:    "test",
					Address: "192.168.0.4",
				},
			},
		},
	})

	bs, _ := json.Marshal(message)
	t.Log(string(bs))
}
