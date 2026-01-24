package wrrp

import (
	"fmt"
	"testing"
)

func TestProtocol(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		header := &Header{
			Version:    1,
			Cmd:        Register,
			PayloadLen: 0,
			FromID:     1,
			ToID:       2,
			Magic:      MagicNumber,
		}

		data := header.Marshal()

		h1, err := Unmarshal(data)
		if err != nil {
			t.Fatal(err)
		}

		fmt.Println(h1)

		data1 := h1.Marshal()
		fmt.Println(data)
		fmt.Println(data1)
	})
}
