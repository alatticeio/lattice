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

	"github.com/alatticeio/lattice/internal/relay"
)

// BenchmarkLRPFrameEncode measures encoding and decoding of the 12-byte LRP
// frame header. Target: < 5 μs per op (both paths should be well under 1 μs).
func BenchmarkLRPFrameEncode(b *testing.B) {
	h := relay.Header{
		Seq:        42,
		PayloadLen: 1400,
		Cmd:        relay.Forward,
		ToID:       0xDEADBEEF,
		Reserved:   0,
	}

	b.Run("Marshal", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		var out []byte
		for i := 0; i < b.N; i++ {
			out = h.Marshal()
		}
		_ = out
	})

	buf := make([]byte, relay.HeaderSize)
	b.Run("MarshalInto", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			h.MarshalInto(buf)
		}
	})

	encoded := h.Marshal()
	b.Run("Unmarshal", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		var hdr *relay.Header
		var err error
		for i := 0; i < b.N; i++ {
			hdr, err = relay.Unmarshal(encoded)
		}
		_, _ = hdr, err
	})
}
