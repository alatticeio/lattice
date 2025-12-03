// Copyright 2025 wireflowio.com, Inc.
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

package turn

import (
	"net"
	"sync"
)

// Client
type Client interface {
	GetRelayInfo(allocated bool) (*RelayInfo, error)
}

type RelayInfo struct {
	MappedAddr net.UDPAddr
	RelayConn  net.PacketConn
}

type TurnManager struct {
	mu        sync.Mutex
	RelayInfo *RelayInfo
}

func (m *TurnManager) GetInfo() *RelayInfo {
	return m.RelayInfo
}

func (m *TurnManager) SetInfo(info *RelayInfo) {
	m.mu.Lock()
	m.RelayInfo = info
	m.mu.Unlock()
}

func AddrToUdpAddr(addr net.Addr) (*net.UDPAddr, error) {
	result, err := net.ResolveUDPAddr("udp", addr.String())
	if err != nil {
		return nil, err
	}

	return result, nil
}
