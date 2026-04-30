// Copyright 2026 The Lattice Authors, Inc.
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

package relay

import (
	"encoding/binary"
	"errors"
)

const HeaderSize = 12

// Commands
const (
	Register  uint8 = 0x01
	Forward   uint8 = 0x02
	KeepAlive uint8 = 0x03
	Probe     uint8 = 0x04
)

// Header is the 12-byte LRP frame header (little-endian).
// Offset 0-1:   Seq        — frame sequence number
// Offset 2-5:   PayloadLen — payload size in bytes
// Offset 6:     Cmd        — command byte
// Offset 7-10:  ToID       — target peer ID (uint32)
// Offset 11:    Reserved   — must be 0
type Header struct {
	Seq        uint16
	PayloadLen uint32
	Cmd        uint8
	ToID       uint32
	Reserved   uint8
}

func (h *Header) Marshal() []byte {
	buf := make([]byte, HeaderSize)
	binary.LittleEndian.PutUint16(buf[0:2], h.Seq)
	binary.LittleEndian.PutUint32(buf[2:6], h.PayloadLen)
	buf[6] = h.Cmd
	binary.LittleEndian.PutUint32(buf[7:11], h.ToID)
	buf[11] = h.Reserved
	return buf
}

// MarshalInto writes the header into an existing buffer (must be >= HeaderSize).
func (h *Header) MarshalInto(buf []byte) {
	binary.LittleEndian.PutUint16(buf[0:2], h.Seq)
	binary.LittleEndian.PutUint32(buf[2:6], h.PayloadLen)
	buf[6] = h.Cmd
	binary.LittleEndian.PutUint32(buf[7:11], h.ToID)
	buf[11] = h.Reserved
}

func Unmarshal(data []byte) (*Header, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("lrp: header too short")
	}
	h := &Header{}
	h.Seq = binary.LittleEndian.Uint16(data[0:2])
	h.PayloadLen = binary.LittleEndian.Uint32(data[2:6])
	h.Cmd = data[6]
	h.ToID = binary.LittleEndian.Uint32(data[7:11])
	h.Reserved = data[11]
	return h, nil
}
