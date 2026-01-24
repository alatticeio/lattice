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

package wrrp

import (
	"encoding/binary"
	"errors"
)

const (
	HeaderSize = 28
	// WRRP 的幻数 (4字节)
	MagicNumber uint32 = 0x57525250 // ASCII: 'W' 'R' 'R' 'P'
)

// 指令定义
const (
	Register uint8 = 0x01 // 客户端注册/握手
	Forward  uint8 = 0x02 // 数据转发
	Ping     uint8 = 0x03 // 心跳检测
	Probe    uint8 = 0x04 // 交换机sessionId信息包
)

// Header WRRP 协议头 (共 28 字节)
type Header struct {
	FromID     uint64 // 0-7  (起始就是 0，天然对齐)
	ToID       uint64
	Magic      uint32
	PayloadLen uint32
	Version    uint8
	Cmd        uint8
	Reserved   uint16
}

func (h *Header) Marshal() []byte {
	bufp := headerPool.Get().(*[]byte)
	defer headerPool.Put(bufp)
	buf := *bufp
	binary.BigEndian.PutUint64(buf[0:8], h.FromID)
	binary.BigEndian.PutUint64(buf[8:16], h.ToID)
	binary.BigEndian.PutUint32(buf[16:20], h.Magic)
	binary.BigEndian.PutUint32(buf[20:24], h.PayloadLen)
	buf[24] = h.Version
	buf[25] = h.Cmd
	binary.BigEndian.PutUint16(buf[26:28], h.Reserved)
	return buf
}

// Unmarshal 从字节流解析 Header
func Unmarshal(data []byte) (*Header, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("header too short")
	}
	h := &Header{}
	h.FromID = binary.BigEndian.Uint64(data[0:8])
	h.ToID = binary.BigEndian.Uint64(data[8:16])
	h.Magic = binary.BigEndian.Uint32(data[16:20])
	if h.Magic != MagicNumber {
		return nil, errors.New("invalid magic number")
	}
	h.PayloadLen = binary.BigEndian.Uint32(data[20:24])

	h.Version = data[24]
	h.Cmd = data[25]
	h.Reserved = binary.BigEndian.Uint16(data[26:28])
	return h, nil
}
