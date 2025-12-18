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
	"wireflow/internal/core/domain"
	"wireflow/pkg/utils"
)

// PeerResolver 根据network与policies来计算当前node的最后要连接peers
// PeerResolver 只关注要连接的对象， 更细粒度的防火墙规则由FileWallResolver来实现
type PeerResolver interface {
	ResolvePeers(ctx context.Context, network *domain.Network, policies []*domain.Policy) ([]*domain.Peer, error)
}

type peerResolver struct {
}

func NewPeerResolver() PeerResolver {
	return &peerResolver{}
}

// ResolvePeers 只计算egress，表示当前节点该连接谁，然后与Network中的Peers做过滤
func (p *peerResolver) ResolvePeers(ctx context.Context, network *domain.Network, policies []*domain.Policy) ([]*domain.Peer, error) {
	//加入到该网络中的所有Peers
	peerSet := peerToSet(network.Peers)

	var (
		peers  []*domain.Peer
		result []*domain.Peer
	)
	//过滤出站
	for _, policy := range policies {
		egresses := policy.Egress
		for _, egress := range egresses {
			peers = append(peers, egress.Peers...)
		}

		peerSetTmp := peerStringSet(peers)
		if len(peers) > 0 {
			peers = utils.Filter(peers, func(peer *domain.Peer) bool {
				if _, ok := peerSetTmp[peer.Name]; !ok {
					return false
				}
				return true
			})
		}
	}

	//build computed peers
	for _, peer := range peers {
		if _, ok := peerSet[peer.Name]; ok {
			result = append(result, peerSet[peer.Name])
		}
	}

	set := make(map[string]struct{})
	result = utils.Filter(result, func(peer *domain.Peer) bool {
		if _, ok := set[peer.Name]; ok {
			return false
		}

		set[peer.Name] = struct{}{}
		return true
	})

	return result, nil
}

func peerStringSet(peers []*domain.Peer) map[string]struct{} {
	m := make(map[string]struct{})
	for _, peer := range peers {
		m[peer.Name] = struct{}{}
	}
	return m
}
