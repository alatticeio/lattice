package drp

import (
	"net"
)

// Node is drp node user created.
type Node struct {
	// NodeID is the node id.
	NodeId string

	// when drp use ip v4, the ip v4 address
	IpV4Addr *net.TCPAddr

	// when drp use ip v6, the ip v6 address
	Ipv6Addr *net.TCPAddr
}

func NewNode(nodeId string, ipV4Addr, ipV6Addr *net.TCPAddr) *Node {
	return &Node{
		NodeId:   nodeId,
		IpV4Addr: ipV4Addr,
		Ipv6Addr: ipV6Addr,
	}
}
