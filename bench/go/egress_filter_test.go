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
	"net/netip"
	"testing"
)

// BenchmarkEgressFilterCheck measures the cost of checking a source IP against
// a CIDR allowlist with 10 rules — the per-packet overhead of Lattice's egress
// policy enforcement. Target: < 1 μs per op.
func BenchmarkEgressFilterCheck(b *testing.B) {
	rules := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParsePrefix("192.168.2.0/24"),
		netip.MustParsePrefix("192.168.3.0/24"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
	}

	// A hit: matches the first rule early (best case).
	hitAddr := netip.MustParseAddr("10.1.2.3")
	// A miss: matches no rule (worst case, must scan all 10).
	missAddr := netip.MustParseAddr("8.8.8.8")

	checkAllowed := func(addr netip.Addr) bool {
		for _, prefix := range rules {
			if prefix.Contains(addr) {
				return true
			}
		}
		return false
	}

	b.Run("hit", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		var result bool
		for i := 0; i < b.N; i++ {
			result = checkAllowed(hitAddr)
		}
		_ = result
	})

	b.Run("miss", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		var result bool
		for i := 0; i < b.N; i++ {
			result = checkAllowed(missAddr)
		}
		_ = result
	})
}
