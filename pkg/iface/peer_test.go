package iface

import (
	"fmt"
	"net/netip"
	"testing"
)

func TestPrefix(t *testing.T) {
	s := "10.0.0.2/32"
	prefix, err := netip.ParsePrefix(s)
	fmt.Println(prefix, err)
}
