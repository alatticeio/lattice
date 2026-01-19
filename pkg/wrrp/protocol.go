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
	HeaderSize = 40
	// WRRP 的幻数 (4字节)
	MagicNumber uint32 = 0x57525250 // ASCII: 'W' 'R' 'R' 'P'
)

// 指令定义
const (
	Register uint8 = 0x01 // 客户端注册/握手
	Forward  uint8 = 0x02 // 数据转发
	Ping     uint8 = 0x03 // 心跳检测
)

// Header WRRP 协议头 (共 40 字节)
type Header struct {
	Magic      uint32
	Version    uint16
	Cmd        uint8
	Reserved   uint8
	PayloadLen uint32
	SessionID  [28]byte
}

// Marshal 将 Header 序列化为字节流
func (h *Header) Marshal() []byte {
	bufp := headerPool.Get().(*[]byte)
	defer headerPool.Put(bufp)
	buf := *bufp
	binary.BigEndian.PutUint32(buf[0:4], h.Magic)
	binary.BigEndian.PutUint16(buf[4:6], h.Version)
	buf[6] = h.Cmd
	buf[7] = h.Reserved
	binary.BigEndian.PutUint32(buf[8:12], h.PayloadLen)
	copy(buf[12:40], h.SessionID[:])
	return buf
}

// Unmarshal 从字节流解析 Header
func Unmarshal(data []byte) (*Header, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("header too short")
	}
	h := &Header{}
	h.Magic = binary.BigEndian.Uint32(data[0:4])
	if h.Magic != MagicNumber {
		return nil, errors.New("invalid magic number")
	}
	h.Version = binary.BigEndian.Uint16(data[4:6])
	h.Cmd = data[6]
	h.Reserved = data[7]
	h.PayloadLen = binary.BigEndian.Uint32(data[8:12])
	copy(h.SessionID[:], data[12:40])
	return h, nil
}
