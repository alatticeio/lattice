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

package infra

import (
	"context"
	"net"
	"net/netip"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Wrrp interface {
	ReceiveFunc() conn.ReceiveFunc
	Send(ctx context.Context, remoteId uint64, wrrpType uint8, data []byte) error
	Connect() error
	RemoteAddr() net.Addr
}

var (
	_ conn.Endpoint = (*WRRPEndpoint)(nil)
)

func IDFromPublicKey(pubKey string) ([32]byte, error) {
	key, err := wgtypes.ParseKey(pubKey)
	if err != nil {
		return [32]byte{}, err
	}
	return key, nil
}

// 1. 自定义一个极简的 Endpoint
type WRRPEndpoint struct {
	// 物理层信息 (用于标准 UDP/ICE)
	Addr netip.AddrPort

	// 协议层信息 (用于 WRRP)
	RemoteId uint64

	// 标志位：当前该走哪条路
	TransportType TransportType
}

func (e *WRRPEndpoint) ClearSrc() {

}

func (e *WRRPEndpoint) Clear() {}
func (e *WRRPEndpoint) DstToString() string {
	return e.Addr.String()
}

func (e *WRRPEndpoint) DstToBytes() []byte {
	//if e.TransportType == WRRP {
	//	b := make([]byte, 8)
	//	binary.BigEndian.PutUint64(b, e.RemoteId)
	//	return b
	//}
	// 标准 UDP 模式下，AddrPort 转换为字节
	b, _ := e.Addr.MarshalBinary()
	return b
}

func (e *WRRPEndpoint) DstIP() netip.Addr {
	return e.Addr.Addr()
}

func (e *WRRPEndpoint) SrcIP() netip.Addr {
	return netip.Addr{}
}
func (e *WRRPEndpoint) SrcToString() string { return "" }
