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
	"wireflow/api/v1alpha1"
	"wireflow/internal/infra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// PeerResolver 根据network与policies来计算当前node的最后要连接peers
// PeerResolver 只关注要连接的对象， 更细粒度的防火墙规则由FileWallResolver来实现
type PeerResolver interface {
	ResolvePeers(ctx context.Context, network *infra.Message, policies []*v1alpha1.WireflowPolicy) ([]*infra.Peer, error)
}

type peerResolver struct {
}

func NewPeerResolver() PeerResolver {
	return &peerResolver{}
}

// ResolvePeers zero trust, add when labeled peer matched
func (p *peerResolver) ResolvePeers(ctx context.Context, msg *infra.Message, policies []*v1alpha1.WireflowPolicy) ([]*infra.Peer, error) {
	return GetComputedPeers(msg.Current, msg.Network, policies), nil
}

// 假设我们要为当前节点 currPeer 生成连接列表
func GetComputedPeers(current *infra.Peer, network *infra.Network, policies []*v1alpha1.WireflowPolicy) []*infra.Peer {
	allPeers := network.Peers
	finalPeersMap := make(map[string]*infra.Peer)

	for _, policy := range policies {
		if !matchLabels(current, &policy.Spec.PeerSelector) {
			continue
		}

		// 2. 处理出站 (Egress): 这些是当前节点主动要连接的目标
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

		// 3. 处理入站 (Ingress):
		// 注意：在对等网络中，如果 A 允许 B 入站，通常意味着 B 需要连接 A。
		// 如果你的逻辑是生成“被动允许列表”，则记录在别处；
		// 如果是生成“全双工连接”，则也需要把 Ingress 节点加入。
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
	}

	// 转换为 Slice 返回
	result := make([]*infra.Peer, 0, len(finalPeersMap))
	for _, p := range finalPeersMap {
		result = append(result, p)
	}

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

func peerStringSet(peers []*infra.Peer) map[string]struct{} {
	m := make(map[string]struct{})
	for _, peer := range peers {
		m[peer.Name] = struct{}{}
	}
	return m
}
