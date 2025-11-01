package drp

import (
	"fmt"
	"net"
	"net/url"
	"testing"
)

func TestParseNode(t *testing.T) {

	str := "http://10.0.0.1:8080/drp"
	u, err := url.Parse(str)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(u.Host, u.Port())

	addr, err := net.ResolveTCPAddr("tcp", u.Host)
	if err != nil {
		t.Fatal(err)
	}

	node := NewNode("", addr, nil)
	fmt.Println(node)

}
