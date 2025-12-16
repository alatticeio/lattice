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
	"wireflow/internal/core/domain"
)

func TestPeerResolver_ResolvePeers(t *testing.T) {
	resolver := NewPeerResolver()

	t.Run("success", func(t *testing.T) {
		peer := &domain.Peer{
			Name: "test",
		}
		var peers []*domain.Peer
		peers = append(peers, peer)
		network := &domain.Network{
			Peers: peers,
		}

		var policies []*domain.Policy
		for i := 0; i < 3; i++ {
			rule := &domain.Rule{
				Peers: []*domain.Peer{peer},
			}

			rules := []*domain.Rule{rule}
			policy := &domain.Policy{
				Ingress: rules,
			}
			policies = append(policies, policy)
		}

		result, err := resolver.ResolvePeers(context.Background(), network, policies)
		t.Log(result, err)
	})
}
