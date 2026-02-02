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

package controller

import (
	"context"
	"testing"
	"wireflow/internal/infra"
)

func TestPeerResolver_ResolvePeers(t *testing.T) {
	resolver := NewPeerResolver()

	t.Run("success", func(t *testing.T) {
		peer := &infra.Peer{
			Name: "test",
		}
		var peers []*infra.Peer
		peers = append(peers, peer)
		network := &infra.Network{
			Peers: peers,
		}

		var policies []*infra.Policy
		for i := 0; i < 3; i++ {
			rule := &infra.TrafficRule{
				Peers: []*infra.Peer{peer},
			}

			rules := []*infra.TrafficRule{rule}
			policy := &infra.Policy{
				Ingress: rules,
			}
			policies = append(policies, policy)
		}

		result, err := resolver.ResolvePeers(context.Background(), network, policies)
		t.Log(result, err)
	})
}
