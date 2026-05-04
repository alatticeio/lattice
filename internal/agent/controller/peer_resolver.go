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

package controller

import (
	"context"
	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// PeerResolver 根据network与policies来计算当前node的最后要连接peers
// PeerResolver 只关注要连接的对象， 更细粒度的防火墙规则由FileWallResolver来实现
type PeerResolver interface {
	ResolvePeers(ctx context.Context, network *infra.Message, policies []*v1alpha1.LatticePolicy) ([]*infra.Peer, error)
}

type peerResolver struct {
}

func NewPeerResolver() PeerResolver {
	return &peerResolver{}
}

// ResolvePeers zero trust, add when labeled peer matched
func (p *peerResolver) ResolvePeers(ctx context.Context, msg *infra.Message, policies []*v1alpha1.LatticePolicy) ([]*infra.Peer, error) {
	return GetComputedPeers(msg.Current, msg.Network, policies), nil
}

// GetComputedPeers determines which peers the current node should establish
// WireGuard tunnels with. When any policy matches the current node, all
// peers in the network are included — policies control firewall rules via
// ComputedRules, not peer connectivity. IPBlock-only rules (which can't
// resolve to specific peers) still need full mesh connectivity.
func GetComputedPeers(current *infra.Peer, network *infra.Network, policies []*v1alpha1.LatticePolicy) []*infra.Peer {
	allPeers := network.Peers
	finalPeersMap := make(map[string]*infra.Peer)

	for _, policy := range policies {
		if !matchLabels(current, &policy.Spec.PeerSelector) {
			continue
		}

		// Process egress: which peers this node connects to
		for _, egress := range policy.Spec.Egress {
			for _, peerSelection := range egress.To {
				matchedPeers := resolveSelectionToPeers(peerSelection, allPeers)
				for _, peer := range matchedPeers {
					if peer.Name != current.Name {
						finalPeersMap[peer.Name] = peer
					}
				}
			}
		}

		// Process ingress: peers that connect to this node
		for _, ingress := range policy.Spec.Ingress {
			for _, p := range ingress.From {
				matchedPeers := resolveSelectionToPeers(p, allPeers)
				for _, peer := range matchedPeers {
					if peer.Name != current.Name {
						finalPeersMap[peer.Name] = peer
					}
				}
			}
		}

		// If this policy matched but no peers were resolved via PeerSelector
		// (e.g. IPBlock-only rules), include all network peers to ensure full
		// mesh connectivity. Policies control firewall rules, not peer topology.
		if len(finalPeersMap) == 0 {
			for _, peer := range allPeers {
				if peer.Name != current.Name {
					finalPeersMap[peer.Name] = peer
				}
			}
		}
	}

	// 转换为 Slice 返回，按 Name 排序保证 hash 稳定
	result := make([]*infra.Peer, 0, len(finalPeersMap))
	for _, p := range finalPeersMap {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

func matchLabels(current *infra.Peer, peerSelector *metav1.LabelSelector) bool {
	selector, _ := metav1.LabelSelectorAsSelector(peerSelector)
	// 1. 检查当前 Policy 是否适用于当前节点 (Selector 匹配)
	if !selector.Matches(labels.Set(current.Labels)) {
		return false
	}

	return true
}

// resolveSelectionToPeers 是核心：根据选择器规则（Labels 等）在全量池中查找
func resolveSelectionToPeers(selection v1alpha1.PeerSelection, allPeers []*infra.Peer) []*infra.Peer {
	var result []*infra.Peer
	for _, p := range allPeers {
		// 这里是关键逻辑：判断节点的 Labels 是否匹配选择器定义
		selector, _ := metav1.LabelSelectorAsSelector(selection.PeerSelector)
		if selector.Matches(labels.Set(p.Labels)) {
			result = append(result, p)
		}
	}
	return result
}

// nolint:all
func peerStringSet(peers []*infra.Peer) map[string]struct{} {
	m := make(map[string]struct{})
	for _, peer := range peers {
		m[peer.Name] = struct{}{}
	}
	return m
}
