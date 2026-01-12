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
	v1alpha1 "wireflow/api/v1alpha1"
	"wireflow/internal/core/infra"

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

// PeerStateSnapshot
type PeerStateSnapshot struct {
	Peer     *v1alpha1.WireflowPeer
	Network  *v1alpha1.WireflowNetwork
	Policies []*v1alpha1.WireflowPolicy
	Peers    []*v1alpha1.WireflowPeer
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
	oldPeerSnapshot *PeerStateSnapshot,
	oldPeer, newPeer *v1alpha1.WireflowPeer,
	oldNetwork, newNetwork *v1alpha1.WireflowNetwork,
	oldPolicies, newPolicies []*v1alpha1.WireflowPolicy,
	req ctrl.Request,
) *infra.ChangeDetails {

	changes := &infra.ChangeDetails{
		TotalChanges: 0,
	}

	//1. 检测节点本身的变化
	d.detectNodeConfigChanges(ctx, changes, oldPeerSnapshot, oldPeer, newPeer, req)

	// 2. 检测网络拓扑变化（peers）
	d.detectNetworkChanges(ctx, changes, oldPeerSnapshot, oldNetwork, newNetwork, req)

	//3. 检测网络策略的变化
	d.detectPolicyChanges(ctx, changes, oldPolicies, newPolicies, req)
	// 4. 生成原因描述
	if changes.Reason == "" {
		changes.Reason = changes.Summary()
	}

	return changes
}

func (d *ChangeDetector) detectNodeConfigChanges(ctx context.Context, changes *infra.ChangeDetails, oldNodeCtx *PeerStateSnapshot, oldNode, newNode *v1alpha1.WireflowPeer, req ctrl.Request) *infra.ChangeDetails {
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
		oldNet, newNet := oldNode.Spec.Network, newNode.Spec.Network
		if newNet != nil {
			if oldNet == nil {
				changes.NetworkJoined = append(changes.NetworkJoined, *newNet)
				changes.TotalChanges++
			}

			if oldNet != nil && *newNet != *oldNet {
				changes.NetworkLeft = append(changes.NetworkLeft, *oldNet)
				changes.TotalChanges++
			}
		}

		return changes
	}

	// 新节点
	changes.Reason = "Peer new created"
	changes.TotalChanges++

	return changes
}

func (d *ChangeDetector) detectNetworkChanges(ctx context.Context, changes *infra.ChangeDetails, oldNodeCtx *PeerStateSnapshot, oldNetwork, newNetwork *v1alpha1.WireflowNetwork, req ctrl.Request) *infra.ChangeDetails {
	networkUpdateType := d.detectNetworkUpdateType(oldNetwork, newNetwork)

	switch networkUpdateType {
	case typeAdd:
		changes.NetworkJoined = []string{newNetwork.Name}
		peers, err := d.findNodes(ctx, newNetwork.Name)
		if err != nil {
			return changes
		}
		changes.PeersAdded = peers
		changes.Reason = "WireflowNetwork new created"
		changes.TotalChanges++
		return changes
	case typeDel:
		changes.NetworkLeft = []string{oldNetwork.Name}
		changes.Reason = "WireflowNetwork deleted"
		peers, err := d.findNodes(ctx, oldNetwork.Name)
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
	//typeUpdate
)

func (d *ChangeDetector) detectNetworkUpdateType(oldNetwork, newNetwork *v1alpha1.WireflowNetwork) changeType {
	if oldNetwork == nil && newNetwork == nil {
		return typeNone
	}
	if oldNetwork == nil && newNetwork != nil {
		return typeAdd
	}

	if oldNetwork != nil && newNetwork == nil {
		return typeDel
	}

	return typeNone
}

func (d *ChangeDetector) detectPolicyUpdateType(oldPolicies, newPolicies []*v1alpha1.WireflowPolicy) changeType {

	if oldPolicies == nil && newPolicies != nil {
		return typeAdd
	}

	if oldPolicies != nil && newPolicies == nil {
		return typeDel
	}

	return typeNone
}

func (d *ChangeDetector) detectPolicyChanges(ctx context.Context, changes *infra.ChangeDetails, oldPolicies, newPolicies []*v1alpha1.WireflowPolicy, req ctrl.Request) *infra.ChangeDetails {

	policyUpdateType := d.detectPolicyUpdateType(oldPolicies, newPolicies)

	switch policyUpdateType {
	case typeAdd:
		changes.Reason = "WireflowPolicy new created"
		policies := make([]*infra.Policy, 0)
		for _, p := range newPolicies {
			policies = append(policies, d.transferToPolicy(ctx, p))
		}
		changes.PoliciesAdded = append(changes.PoliciesAdded, policies...)
		changes.TotalChanges++
		return changes
	case typeDel:
		changes.Reason = "WireflowPolicy deleted"
		policies := make([]*infra.Policy, 0)
		for _, p := range oldPolicies {
			policies = append(policies, d.transferToPolicy(ctx, p))
		}

		changes.PoliciesRemoved = append(changes.PoliciesRemoved, policies...)
		changes.TotalChanges++
		return changes
	}

	return changes
}

func (d *ChangeDetector) findPolicy(ctx context.Context, node *v1alpha1.WireflowPeer, req ctrl.Request) ([]*infra.Policy, error) {
	var policyList v1alpha1.WireflowPolicyList
	if err := d.client.List(ctx, &policyList, client.InNamespace(req.Namespace)); err != nil {
		return nil, err
	}

	var policies []*infra.Policy

	for _, policy := range policyList.Items {
		selector, _ := metav1.LabelSelectorAsSelector(&policy.Spec.PeerSelector)
		matched := selector.Matches(labels.Set(node.Labels))
		if matched {
			p := d.transferToPolicy(ctx, &policy)
			policies = append(policies, p)
		}
	}

	return policies, nil
}

func (d *ChangeDetector) findNodes(ctx context.Context, networkName string) ([]*infra.Peer, error) {
	log := logf.FromContext(ctx)
	log.Info("findNodes by network labels", "networkName", networkName)

	labels := map[string]string{
		"wireflow.run/network-": networkName,
	}

	var peers v1alpha1.WireflowPeerList
	if err := d.client.List(ctx, &peers, client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	var addedPeers []*infra.Peer
	for _, item := range peers.Items {
		addedPeers = append(addedPeers, transferToPeer(&item))
	}

	return addedPeers, nil
}

func (d *ChangeDetector) generateConfigmap(ctx context.Context, current *v1alpha1.WireflowPeer, snapshot *PeerStateSnapshot, changes *infra.ChangeDetails, version string) (*infra.Message, error) {
	var err error
	// 生成配置版本号
	msg := &infra.Message{
		EventType:     infra.EventTypeNodeUpdate, // 统一使用 ConfigUpdate
		ConfigVersion: version,
		Timestamp:     time.Now().Unix(),
		Changes:       changes, // ← 携带变更详情
		Current:       transferToPeer(current),
		Network: &infra.Network{
			Peers: make([]*infra.Peer, 0),
		},
	}

	// 填充网络信息
	if snapshot.Network != nil {
		msg.Network.NetworkId = snapshot.Network.Name
		msg.Network.NetworkName = snapshot.Network.Spec.Name

		// 填充 peers
		for _, p := range snapshot.Peers {
			if p.Status.AllocatedAddress == nil {
				continue
			}

			msg.Network.Peers = append(msg.Network.Peers, transferToPeer(p))
		}
	}

	if len(snapshot.Policies) > 0 {
		// 填充策略
		for _, policy := range snapshot.Policies {
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

func peerToSet(peers []*infra.Peer) map[string]*infra.Peer {
	m := make(map[string]*infra.Peer)
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

func (d *ChangeDetector) transferToPolicy(ctx context.Context, src *v1alpha1.WireflowPolicy) *infra.Policy {
	log := logf.FromContext(ctx)
	log.Info("transferToPolicy", "policy", src.Name)
	policy := &infra.Policy{
		PolicyName: src.Name,
	}

	var ingresses, egresses []*infra.Rule
	srcIngresses := src.Spec.IngressRule
	srcEgresses := src.Spec.EgressRule
	for _, ingress := range srcIngresses {
		rule := &infra.Rule{}
		nodes, err := d.getPeerFromLabels(ctx, ingress.From)
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
		rule := &infra.Rule{}
		nodes, err := d.getPeerFromLabels(ctx, egress.To)
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

func (d *ChangeDetector) getPeerFromLabels(ctx context.Context, rules []v1alpha1.PeerSelection) ([]*infra.Peer, error) {
	// 使用 map 来存储已找到的节点，以确保结果不重复
	// key: 节点的 UID，value: 节点对象本身
	foundNodes := make(map[types.UID]v1alpha1.WireflowPeer)

	for _, rule := range rules {
		// 1. 将 metav1.LabelSelector 转换为 labels.Selector 接口
		selector, err := metav1.LabelSelectorAsSelector(rule.PeerSelector)
		if err != nil {
			// 记录错误，无法解析选择器
			return nil, fmt.Errorf("failed to parse label selector %v: %w", rule.PeerSelector, err)
		}

		var nodeList v1alpha1.WireflowPeerList

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
	result := make([]*infra.Peer, 0, len(foundNodes))
	for _, node := range foundNodes {
		result = append(result, transferToPeer(&node))
	}

	return result, nil
}
