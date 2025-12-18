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
	"fmt"
	"sync"
	"time"
	wireflowv1alpha1 "wireflow/api/v1alpha1"
	"wireflow/internal/core/domain"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ChangeDetector struct {
	client           client.Client
	versionMu        sync.Mutex
	versionCounter   int64
	peerResolver     PeerResolver
	firewallResolver FirewallRuleResolver
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
		client:           client,
		peerResolver:     NewPeerResolver(),
		firewallResolver: NewFirewallResolver(),
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
) *domain.ChangeDetails {

	changes := &domain.ChangeDetails{
		TotalChanges: 0,
	}

	//1. 检测节点本身的变化
	d.detectNodeConfigChanges(ctx, changes, oldNodeCtx, oldNode, newNode, req)

	// 2. 检测网络拓扑变化（peers）
	d.detectNetworkChanges(ctx, changes, oldNodeCtx, oldNetwork, newNetwork, req)

	//3. 检测网络策略的变化
	d.detectPolicyChanges(ctx, changes, oldPolicies, newPolicies, req)
	// 4. 生成原因描述
	if changes.Reason == "" {
		changes.Reason = changes.Summary()
	}

	return changes
}

func (d *ChangeDetector) detectNodeConfigChanges(ctx context.Context, changes *domain.ChangeDetails, oldNodeCtx *NodeContext, oldNode, newNode *wireflowv1alpha1.Node, req ctrl.Request) *domain.ChangeDetails {
	var newCreated bool
	if oldNode == nil {
		newCreated = true
	}
	// 1. 检测节点自身变化
	if !newCreated {
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

		return changes
	}

	// 新节点
	changes.Reason = "Peer new created"
	changes.TotalChanges++

	return changes
}

func (d *ChangeDetector) detectNetworkChanges(ctx context.Context, changes *domain.ChangeDetails, oldNodeCtx *NodeContext, oldNetwork, newNetwork *wireflowv1alpha1.Network, req ctrl.Request) *domain.ChangeDetails {
	networkUpdateType := d.detectNetworkUpdateType(oldNetwork, newNetwork)

	switch networkUpdateType {
	case typeUpdate:
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
		return changes
	case typeAdd:
		changes.NetworkJoined = []string{newNetwork.Name}
		peers, err := d.findNodes(ctx, newNetwork.Spec.Nodes, req)
		if err != nil {
			return changes
		}
		changes.PeersAdded = peers
		changes.Reason = "Network new created"
		changes.TotalChanges++
		return changes
	case typeDel:
		changes.NetworkLeft = []string{oldNetwork.Name}
		changes.Reason = "Network deleted"
		peers, err := d.findNodes(ctx, oldNetwork.Spec.Nodes, req)
		if err != nil {
			return changes
		}
		changes.PeersRemoved = peers
		changes.TotalChanges++
		return changes
	}

	return changes
}

type changeType int

const (
	typeNone changeType = iota
	typeAdd
	typeDel
	typeUpdate
)

func (d *ChangeDetector) detectNetworkUpdateType(oldNetwork, newNetwork *wireflowv1alpha1.Network) changeType {
	if oldNetwork == nil && newNetwork == nil {
		return typeNone
	}
	if oldNetwork == nil && newNetwork != nil {
		return typeAdd
	}

	if oldNetwork != nil && newNetwork == nil {
		return typeDel
	}

	if oldNetwork.Spec.CIDR != newNetwork.Spec.CIDR {
		return typeUpdate
	}

	return typeNone
}

func (d *ChangeDetector) detectPolicyUpdateType(oldPolicies, newPolicies []*wireflowv1alpha1.NetworkPolicy) changeType {

	if oldPolicies == nil && newPolicies != nil {
		return typeAdd
	}

	if oldPolicies != nil && newPolicies == nil {
		return typeDel
	}

	if len(oldPolicies) != len(newPolicies) {
		return typeUpdate
	}

	for i := range oldPolicies {
		if oldPolicies[i].ResourceVersion != newPolicies[i].ResourceVersion {
			return typeUpdate
		}
	}

	return typeNone
}

func (d *ChangeDetector) detectPolicyChanges(ctx context.Context, changes *domain.ChangeDetails, oldPolicies, newPolicies []*wireflowv1alpha1.NetworkPolicy, req ctrl.Request) *domain.ChangeDetails {

	policyUpdateType := d.detectPolicyUpdateType(oldPolicies, newPolicies)

	switch policyUpdateType {
	case typeUpdate:
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
				var policy wireflowv1alpha1.NetworkPolicy
				if err := d.client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: name}, &policy); err != nil {
					return changes
				}
				changes.PoliciesAdded = append(changes.PoliciesAdded, d.transferToPolicy(ctx, &policy))
			}
		}

		// 删除的策略
		for name := range oldPolicyMap {
			if _, exists := newPolicyMap[name]; !exists {
				var policy wireflowv1alpha1.NetworkPolicy
				if err := d.client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: name}, &policy); err != nil {
					return changes
				}
				changes.PoliciesRemoved = append(changes.PoliciesRemoved, d.transferToPolicy(ctx, &policy))
			}
		}

		// 更新的策略（比较 ResourceVersion）
		for name, newPolicy := range newPolicyMap {
			if oldPolicy, exists := oldPolicyMap[name]; exists {
				if oldPolicy.ResourceVersion != newPolicy.ResourceVersion {
					changes.PoliciesUpdated = append(changes.PoliciesUpdated, d.transferToPolicy(ctx, newPolicy))
				}
			}
		}

		if len(changes.PoliciesAdded) > 0 ||
			len(changes.PoliciesRemoved) > 0 ||
			len(changes.PoliciesUpdated) > 0 {
			changes.TotalChanges++
		}
		return changes
	case typeAdd:
		changes.Reason = "NetworkPolicy new created"
		policies := make([]*domain.Policy, 0)
		for _, p := range newPolicies {
			policies = append(policies, d.transferToPolicy(ctx, p))
		}
		changes.PoliciesAdded = append(changes.PoliciesAdded, policies...)
		changes.TotalChanges++
		return changes
	case typeDel:
		changes.Reason = "NetworkPolicy deleted"
		policies := make([]*domain.Policy, 0)
		for _, p := range oldPolicies {
			policies = append(policies, d.transferToPolicy(ctx, p))
		}

		changes.PoliciesRemoved = append(changes.PoliciesRemoved, policies...)
		changes.TotalChanges++
		return changes
	}

	return changes
}

func (d *ChangeDetector) findPolicy(ctx context.Context, node *wireflowv1alpha1.Node, req ctrl.Request) ([]*domain.Policy, error) {
	var policyList wireflowv1alpha1.NetworkPolicyList
	if err := d.client.List(ctx, &policyList, client.InNamespace(req.Namespace)); err != nil {
		return nil, err
	}

	var policies []*domain.Policy

	for _, policy := range policyList.Items {
		selector, _ := metav1.LabelSelectorAsSelector(&policy.Spec.NodeSelector)
		matched := selector.Matches(labels.Set(node.Labels))
		if matched {
			p := d.transferToPolicy(ctx, &policy)
			policies = append(policies, p)
		}
	}

	return policies, nil
}

func (d *ChangeDetector) findChangedNodes(ctx context.Context, oldNodeCtx *NodeContext, added, removed []string, req ctrl.Request) ([]*domain.Peer, []*domain.Peer) {
	logger := logf.FromContext(ctx)
	addedPeers := make([]*domain.Peer, 0)
	removedPeers := make([]*domain.Peer, 0)

	//1、删除的节点
	for _, remove := range removed {
		for _, node := range oldNodeCtx.Nodes {
			if remove == node.Name {
				removedPeers = append(removedPeers, &domain.Peer{
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

		addedPeers = append(addedPeers, &domain.Peer{
			Name:       node.Name,
			AppID:      node.Spec.AppId,
			Address:    node.Status.AllocatedAddress,
			PublicKey:  node.Spec.PublicKey,
			AllowedIPs: fmt.Sprintf("%s/32", node.Status.AllocatedAddress),
		})
	}

	return addedPeers, removedPeers
}

func (d *ChangeDetector) findNodes(ctx context.Context, names []string, req ctrl.Request) ([]*domain.Peer, error) {
	log := logf.FromContext(ctx)
	log.Info("findNodes from", "names", names)
	var addedPeers []*domain.Peer
	for _, name := range names {
		var node wireflowv1alpha1.Node
		if err := d.client.Get(ctx, types.NamespacedName{
			Namespace: req.Namespace,
			Name:      name,
		}, &node); err != nil {
			if errors.IsNotFound(err) {
				log.Info("node not found, may be deleted", "node", name)
			}
			return nil, err
		}

		addedPeers = append(addedPeers, &domain.Peer{
			Name:       node.Name,
			AppID:      node.Spec.AppId,
			Address:    node.Status.AllocatedAddress,
			PublicKey:  node.Spec.PublicKey,
			AllowedIPs: fmt.Sprintf("%s/32", node.Status.AllocatedAddress),
		})
	}

	return addedPeers, nil
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

func (d *ChangeDetector) buildFullConfig(ctx context.Context, node *wireflowv1alpha1.Node, context *NodeContext, changes *domain.ChangeDetails, version string) (*domain.Message, error) {
	var err error
	// 生成配置版本号
	msg := &domain.Message{
		EventType:     domain.EventTypeNodeUpdate, // 统一使用 ConfigUpdate
		ConfigVersion: version,
		Timestamp:     time.Now().Unix(),
		Changes:       changes, // ← 携带变更详情
		Current:       transferToPeer(node),
		Network: &domain.Network{
			Peers: make([]*domain.Peer, 0),
		},
	}

	// 填充网络信息
	if context.Network != nil {
		msg.Network.NetworkId = context.Network.Name
		msg.Network.NetworkName = context.Network.Spec.Name

		// 填充 peers
		for _, peer := range context.Nodes {
			if peer.Status.AllocatedAddress == "" {
				continue
			}
		}
	}

	if len(context.Policies) > 0 {
		// 填充策略
		for _, policy := range context.Policies {
			msg.Policies = append(msg.Policies, d.transferToPolicy(ctx, policy))
		}
	}

	msg.ComputedPeers, err = d.peerResolver.ResolvePeers(ctx, msg.Network, msg.Policies)
	if err != nil {
		return nil, err
	}

	msg.ComputedRules, err = d.firewallResolver.ResolveRules(ctx, msg.Current, msg.Network, msg.Policies)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func peerToSet(peers []*domain.Peer) map[string]*domain.Peer {
	m := make(map[string]*domain.Peer)
	for _, peer := range peers {
		m[peer.Name] = peer
	}

	return m
}

// generateConfigVersion 生成配置版本号
func (d *ChangeDetector) generateConfigVersion() string {
	d.versionMu.Lock()
	defer d.versionMu.Unlock()

	d.versionCounter++
	return fmt.Sprintf("v%d", d.versionCounter)
}

func (d *ChangeDetector) transferToPolicy(ctx context.Context, src *wireflowv1alpha1.NetworkPolicy) *domain.Policy {
	log := logf.FromContext(ctx)
	log.Info("transferToPolicy", "policy", src.Name)
	policy := &domain.Policy{
		PolicyName: src.Name,
	}

	var ingresses, egresses []*domain.Rule
	srcIngresses := src.Spec.IngressRule
	srcEgresses := src.Spec.EgressRule
	for _, ingress := range srcIngresses {
		rule := &domain.Rule{}
		nodes, err := d.getNodeFromLabels(ctx, ingress.From)
		if err != nil {
			log.Error(err, "failed to get nodes from labels", "labels", ingress.From)
			continue
		}

		rule.Peers = nodes

		if len(ingress.Ports) > 0 {
			rule.Protocol = ingress.Ports[0].Protocol
			rule.Port = fmt.Sprintf("%d", ingress.Ports[0].Port)
		}
		ingresses = append(ingresses, rule)
	}

	for _, egress := range srcEgresses {
		rule := &domain.Rule{}
		nodes, err := d.getNodeFromLabels(ctx, egress.To)
		if err != nil {
			log.Error(err, "failed to get nodes from labels", "labels", egress.To)
			continue
		}

		rule.Peers = nodes
		if len(egress.Ports) > 0 {
			rule.Protocol = egress.Ports[0].Protocol
			rule.Port = fmt.Sprintf("%d", egress.Ports[0].Port)
		}
		egresses = append(egresses, rule)
	}

	policy.Ingress = ingresses
	policy.Egress = egresses

	return policy
}

func (d *ChangeDetector) getNodeFromLabels(ctx context.Context, rules []wireflowv1alpha1.PeerSelection) ([]*domain.Peer, error) {
	// 使用 map 来存储已找到的节点，以确保结果不重复
	// key: 节点的 UID，value: 节点对象本身
	foundNodes := make(map[types.UID]wireflowv1alpha1.Node)

	for _, rule := range rules {
		// 1. 将 metav1.LabelSelector 转换为 labels.Selector 接口
		selector, err := metav1.LabelSelectorAsSelector(rule.PeerSelector)
		if err != nil {
			// 记录错误，无法解析选择器
			return nil, fmt.Errorf("failed to parse label selector %v: %w", rule.PeerSelector, err)
		}

		var nodeList wireflowv1alpha1.NodeList

		// 2. 针对每一个选择器执行一次独立的 List API 调用
		// 这实现了 OR 逻辑（匹配选择器 A 的节点集合 + 匹配选择器 B 的节点集合）
		// 确保 ListOptions 放在最后
		if err := d.client.List(ctx, &nodeList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
			// 记录 API 调用错误
			return nil, fmt.Errorf("failed to list nodes with selector %s: %w", selector.String(), err)
		}

		// 3. 将本次查询到的节点添加到 map 中，通过 UID 避免重复
		for _, node := range nodeList.Items {
			// 复制节点对象，避免在后续操作中意外修改
			foundNodes[node.UID] = node
		}
	}

	// 4. 将 map 中的节点转换为切片作为最终结果返回
	result := make([]*domain.Peer, 0, len(foundNodes))
	for _, node := range foundNodes {
		result = append(result, transferToPeer(&node))
	}

	return result, nil
}
