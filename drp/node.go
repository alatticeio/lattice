// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package drp

import (
	"net"
)

// Node is drp node user created.
type Node struct {
	// NodeId is the node id.
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
