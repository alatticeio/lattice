package vo

import (
	"encoding/json"
	"linkany/internal"
	"testing"
)

func TestNewMessage(t *testing.T) {
	// a node event

	message := internal.NewMessage().AddGroup(1, "test")
	message.AddNode(&internal.NodeMessage{
		ID:   1,
		Name: "test",
	})
	bs, _ := json.Marshal(message)
	t.Log(string(bs))
}
