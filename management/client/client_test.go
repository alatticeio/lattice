package client

import (
	"encoding/hex"
	"fmt"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestKey(t *testing.T) {
	k, err := wgtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(k.String())
	dst := hex.EncodeToString(k[:])
	fmt.Println(dst)

	src, err := hex.DecodeString(dst)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(src)
}
