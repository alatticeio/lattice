// Copyright 2025 Wireflow.io, Inc.
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

package resource

import (
	"wireflow/internal"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
)

type ChangeDetector struct {
}

// NodeContext
type NodeContext struct {
	Node     *wireflowv1alpha1.Node
	Network  *wireflowv1alpha1.Network
	Policies []*wireflowv1alpha1.NetworkPolicy
	Nodes    []*wireflowv1alpha1.Node
}

func NewChangeDetector() *ChangeDetector {
	return &ChangeDetector{}
}

// DetectNodeChanges 检测 Node 的所有变化
func (d *ChangeDetector) DetectNodeChanges(
	oldNode, newNode *wireflowv1alpha1.Node,
	oldNetwork, newNetwork *wireflowv1alpha1.Network,
	oldPolicies, newPolicies []*wireflowv1alpha1.NetworkPolicy,
) *internal.ChangeDetails {

	changes := &internal.ChangeDetails{
		TotalChanges: 0,
	}

	// 1. 检测节点自身变化
	if oldNode != nil {
		// IP 地址变化
		if oldNode.Spec.Address != newNode.Spec.Address {
			changes.AddressChanged = true
			changes.TotalChanges++
		}

		// 密钥变化
		if oldNode.Spec.PublicKey != newNode.Spec.PublicKey {
			changes.KeyChanged = true
			changes.TotalChanges++
		}

		// 网络归属变化
		oldNetworks := stringSet(oldNode.Spec.Network)
		newNetworks := stringSet(newNode.Spec.Network)

		changes.NetworkJoined = setDifference(newNetworks, oldNetworks)
		changes.NetworkLeft = setDifference(oldNetworks, newNetworks)

		if len(changes.NetworkJoined) > 0 || len(changes.NetworkLeft) > 0 {
			changes.TotalChanges++
		}
	} else {
		// 新节点
		changes.Reason = "Node created"
		changes.TotalChanges++
	}

	// 2. 检测网络拓扑变化（peers）
	if oldNetwork != nil && newNetwork != nil {
		oldPeers := stringSet(oldNetwork.Spec.Nodes)
		newPeers := stringSet(newNetwork.Spec.Nodes)

		changes.NodesAdded = setDifference(newPeers, oldPeers)
		changes.NodesRemoved = setDifference(oldPeers, newPeers)

		// 检测现有 peer 的更新（需要更详细的比较）
		// 这里简化处理

		if len(changes.NodesAdded) > 0 || len(changes.NodesRemoved) > 0 {
			changes.TotalChanges++
		}

		// 网络配置变化
		if oldNetwork.Spec.CIDR != newNetwork.Spec.CIDR {
			changes.NetworkConfigChanged = true
			changes.TotalChanges++
		}
	}

	// 3. 检测策略变化
	if oldPolicies != nil && newPolicies != nil {
		oldPolicyMap := make(map[string]*wireflowv1alpha1.NetworkPolicy)
		newPolicyMap := make(map[string]*wireflowv1alpha1.NetworkPolicy)

		for _, p := range oldPolicies {
			oldPolicyMap[p.Name] = p
		}
		for _, p := range newPolicies {
			newPolicyMap[p.Name] = p
		}

		// 新增的策略
		for name := range newPolicyMap {
			if _, exists := oldPolicyMap[name]; !exists {
				changes.PoliciesAdded = append(changes.PoliciesAdded, name)
			}
		}

		// 删除的策略
		for name := range oldPolicyMap {
			if _, exists := newPolicyMap[name]; !exists {
				changes.PoliciesRemoved = append(changes.PoliciesRemoved, name)
			}
		}

		// 更新的策略（比较 ResourceVersion）
		for name, newPolicy := range newPolicyMap {
			if oldPolicy, exists := oldPolicyMap[name]; exists {
				if oldPolicy.ResourceVersion != newPolicy.ResourceVersion {
					changes.PoliciesUpdated = append(changes.PoliciesUpdated, name)
				}
			}
		}

		if len(changes.PoliciesAdded) > 0 ||
			len(changes.PoliciesRemoved) > 0 ||
			len(changes.PoliciesUpdated) > 0 {
			changes.TotalChanges++
		}
	}

	// 4. 生成原因描述
	if changes.Reason == "" {
		changes.Reason = changes.Summary()
	}

	return changes
}

// 工具函数

func stringSet(slice []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, s := range slice {
		set[s] = struct{}{}
	}
	return set
}

func setDifference(a, b map[string]struct{}) []string {
	diff := make([]string, 0)
	for k := range a {
		if _, exists := b[k]; !exists {
			diff = append(diff, k)
		}
	}
	return diff
}
