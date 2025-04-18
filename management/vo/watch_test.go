package vo

import (
	"encoding/json"
	"linkany/management/utils"
	"testing"
)

func TestNewMessage(t *testing.T) {
	// a node event

	message := utils.NewMessage().AddGroup(1, "test")
	message.AddNode(&utils.NodeMessage{
		ID:   1,
		Name: "test",
	})
	bs, _ := json.Marshal(message)
	t.Log(string(bs))
}
