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

package benchgo

import (
	"testing"

	"github.com/pion/stun/v3"
)

// BenchmarkFilteringUDPMux measures the cost of the STUN/WireGuard packet
// classification in FilteringUDPMux.readLoop — specifically stun.IsMessage().
// This function is called for every UDP packet received on the shared socket.
// Target: < 0.5 μs per op.
func BenchmarkFilteringUDPMux(b *testing.B) {
	// A minimal valid STUN Binding Request (20-byte header, magic cookie at bytes 4-7).
	stunPkt := []byte{
		0x00, 0x01, // STUN Binding Request
		0x00, 0x00, // message length = 0
		0x21, 0x12, 0xa4, 0x42, // magic cookie
		0x00, 0x00, 0x00, 0x00, // transaction ID (12 bytes)
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}

	// A WireGuard data packet (first byte 0x04 = transport message type).
	wgPkt := make([]byte, 60)
	wgPkt[0] = 0x04

	b.Run("STUN", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		var result bool
		for i := 0; i < b.N; i++ {
			result = stun.IsMessage(stunPkt)
		}
		_ = result
	})

	b.Run("WireGuard", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		var result bool
		for i := 0; i < b.N; i++ {
			result = stun.IsMessage(wgPkt)
		}
		_ = result
	})
}
