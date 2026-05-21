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
	"fmt"
	"testing"

	"github.com/alatticeio/lattice/internal/agent/provision"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// BenchmarkSandboxProvisioner measures the cost of generating WireGuard UAPI
// configuration strings for 100 peers — the provisioning overhead when a
// sandbox network is fully populated.
// Target: < 10 ms per op (the whole 100-peer batch).
func BenchmarkSandboxProvisioner(b *testing.B) {
	// Pre-generate 100 keypairs so key generation is not part of the benchmark.
	peers := make([]*provision.SetPeer, 100)
	for i := range peers {
		key, err := wgtypes.GenerateKey()
		if err != nil {
			b.Fatal(err)
		}
		psk, err := wgtypes.GenerateKey()
		if err != nil {
			b.Fatal(err)
		}
		peers[i] = &provision.SetPeer{
			PublicKey:            key.PublicKey().String(),
			PresharedKey:         psk.String(),
			Endpoint:             fmt.Sprintf("10.0.%d.%d:51820", i/256, i%256),
			AllowedIPs:           fmt.Sprintf("100.64.%d.%d/32", i/256, i%256),
			PersistentKeepalived: 25,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, p := range peers {
			_ = p.String()
		}
	}
}
