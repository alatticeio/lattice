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

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wireflow/internal"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ChangeDetector struct {
	client         client.Client
	versionMu      sync.Mutex
	versionCounter int64
}

// NodeContext
type NodeContext struct {
	Node     *wireflowv1alpha1.Node
	Network  *wireflowv1alpha1.Network
	Policies []*wireflowv1alpha1.NetworkPolicy
	Nodes    []*wireflowv1alpha1.Node
}

func NewChangeDetector(client client.Client) *ChangeDetector {
	return &ChangeDetector{
		client: client,
	}
}

// DetectNodeChanges 检测 Peer 的所有变化
func (d *ChangeDetector) DetectNodeChanges(
	ctx context.Context,
	oldNodeCtx *NodeContext,
	oldNode, newNode *wireflowv1alpha1.Node,
	oldNetwork, newNetwork *wireflowv1alpha1.Network,
	oldPolicies, newPolicies []*wireflowv1alpha1.NetworkPolicy,
	req ctrl.Request,
) *internal.ChangeDetails {

	changes := &internal.ChangeDetails{
		TotalChanges: 0,
	}

	// 1. 检测节点自身变化
	if oldNode != nil {
		// IP 地址变化
		if oldNode.Status.AllocatedAddress != newNode.Status.AllocatedAddress {
			changes.AddressChanged = true
			changes.TotalChanges++
		}

		// 密钥变化
		if oldNode.Spec.PublicKey != newNode.Spec.PublicKey {
			changes.KeyChanged = true
			changes.TotalChanges++
		}

		if oldNode.Spec.PrivateKey != newNode.Spec.PrivateKey {
			changes.KeyChanged = true
			changes.TotalChanges++
		}

		// 网络归属变化
		oldNetworks := stringSet(oldNode.Spec.Networks)
		newNetworks := stringSet(newNode.Spec.Networks)

		changes.NetworkJoined = setDifference(newNetworks, oldNetworks)
		changes.NetworkLeft = setDifference(oldNetworks, newNetworks)

		if len(changes.NetworkJoined) > 0 || len(changes.NetworkLeft) > 0 {
			changes.TotalChanges++
		}
	} else {
		// 新节点
		changes.Reason = "Peer created"
		changes.TotalChanges++
	}

	// 2. 检测网络拓扑变化（peers）
	if oldNetwork != nil && newNetwork != nil {
		oldPeers := stringSet(oldNetwork.Spec.Nodes)
		newPeers := stringSet(newNetwork.Spec.Nodes)

		removed, added := setDifference(oldPeers, newPeers), setDifference(newPeers, oldPeers)
		changes.PeersAdded, changes.PeersRemoved = d.findChangedNodes(ctx, oldNodeCtx, added, removed, req)

		// 检测现有 peer 的更新（需要更详细的比较）
		// 这里简化处理

		if len(changes.PeersAdded) > 0 || len(changes.PeersRemoved) > 0 {
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

func (d *ChangeDetector) findChangedNodes(ctx context.Context, oldNodeCtx *NodeContext, added, removed []string, req ctrl.Request) ([]*internal.Peer, []*internal.Peer) {
	logger := logf.FromContext(ctx)
	addedPeers := make([]*internal.Peer, 0)
	removedPeers := make([]*internal.Peer, 0)

	//1、删除的节点
	for _, remove := range removed {
		for _, node := range oldNodeCtx.Nodes {
			if remove == node.Name {
				removedPeers = append(removedPeers, &internal.Peer{
					Name:       node.Name,
					AppID:      node.Spec.AppId,
					Address:    node.Status.AllocatedAddress,
					PublicKey:  node.Spec.PublicKey,
					AllowedIPs: fmt.Sprintf("%s/32", node.Status.AllocatedAddress),
				})
			}
		}
	}

	for _, name := range added {
		var node wireflowv1alpha1.Node
		if err := d.client.Get(ctx, types.NamespacedName{
			Namespace: req.Namespace,
			Name:      name,
		}, &node); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("node not found, may be deleted", "node", name)
			}
		}

		addedPeers = append(addedPeers, &internal.Peer{
			Name:       node.Name,
			AppID:      node.Spec.AppId,
			Address:    node.Status.AllocatedAddress,
			PublicKey:  node.Spec.PublicKey,
			AllowedIPs: fmt.Sprintf("%s/32", node.Status.AllocatedAddress),
		})
	}

	return addedPeers, removedPeers
}

func (r *NodeReconciler) getNodeContext(ctx context.Context, node *wireflowv1alpha1.Node, req ctrl.Request) *NodeContext {
	if node == nil {
		return &NodeContext{}
	}

	var (
		err error
	)
	nodeCtx := &NodeContext{
		Node: node,
	}

	// 获取网络信息
	if len(node.Spec.Networks) > 0 {
		networkName := node.Spec.Networks[0]
		var network wireflowv1alpha1.Network
		if err = r.Get(ctx, types.NamespacedName{
			Namespace: req.Namespace, Name: networkName,
		}, &network); err != nil {
			return nodeCtx
		}
		if err == nil {
			nodeCtx.Network = &network

			// 获取 peers
			for _, nodeName := range network.Spec.Nodes {
				if nodeName == node.Name {
					continue
				}
				var tmpNode wireflowv1alpha1.Node
				if err = r.Get(ctx, types.NamespacedName{
					Namespace: req.Namespace, Name: nodeName,
				}, &tmpNode); err != nil {
					return nodeCtx
				}

				nodeCtx.Nodes = append(nodeCtx.Nodes, &tmpNode)

			}

			// 获取策略
			//policies, err := n.clientSet.WireflowcontrollerV1alpha1().
			//	NetworkPolicies(node.Namespace).
			//	List(context.Background(), metav1.ListOptions{
			//		LabelSelector: fmt.Sprintf("wireflow.io/network=%s", networkName),
			//	})

			//policies, err := policyLister.NetworkPolicies(node.Namespace).List(labels.Everything())
			//if err == nil {
			//	ctx.Policies = append(ctx.Policies, policies...)
			//}
		}
	}

	return nodeCtx
}

// setDifference returns the elements in a that are not present in b.
func setDifference(a, b map[string]struct{}) []string {
	diff := make([]string, 0)
	for k := range a {
		if _, exists := b[k]; !exists {
			diff = append(diff, k)
		}
	}
	return diff
}

func (d *ChangeDetector) buildFullConfig(node *wireflowv1alpha1.Node, context *NodeContext, changes *internal.ChangeDetails, version string) (*internal.Message, error) {
	// 生成配置版本号
	msg := &internal.Message{
		EventType:     internal.EventTypeNodeUpdate, // 统一使用 ConfigUpdate
		ConfigVersion: version,
		Timestamp:     time.Now().Unix(),
		Changes:       changes, // ← 携带变更详情
		Current: &internal.Peer{
			Name:       node.Name,
			AppID:      node.Spec.AppId,
			Address:    node.Status.AllocatedAddress,
			PublicKey:  node.Spec.PublicKey,
			PrivateKey: node.Spec.PrivateKey,
			//AllowedIPs: node.Spec.AllowIedPS,
		},
		Network: &internal.Network{
			Peers:    make([]*internal.Peer, 0),
			Policies: make([]*internal.Policy, 0),
		},
	}

	// 填充网络信息
	if context.Network != nil {
		msg.Network.NetworkId = context.Network.Name
		msg.Network.NetworkName = context.Network.Spec.Name
		//msg.Network.Address = context.Network.Spec.Address
		//msg.Network.Port = context.Network.Spec.Port

		// 填充 peers
		for _, peer := range context.Nodes {
			if peer.Status.AllocatedAddress == "" {
				continue
			}

			msg.Network.Peers = append(msg.Network.Peers, &internal.Peer{
				Name:       peer.Name,
				AppID:      peer.Spec.AppId,
				Address:    peer.Status.AllocatedAddress,
				PublicKey:  peer.Spec.PublicKey,
				AllowedIPs: fmt.Sprintf("%s/32", peer.Status.AllocatedAddress),
				//Endpoint:   peer.Spec.Endpoint,
			})
		}

		// 填充策略
		for _, policy := range context.Policies {
			msg.Network.Policies = append(msg.Network.Policies, &internal.Policy{
				PolicyName: policy.Name,
				// 填充规则
			})
		}
	}

	return msg, nil
}

// generateConfigVersion 生成配置版本号
func (d *ChangeDetector) generateConfigVersion() string {
	d.versionMu.Lock()
	defer d.versionMu.Unlock()

	d.versionCounter++
	return fmt.Sprintf("v%d", d.versionCounter)
}
