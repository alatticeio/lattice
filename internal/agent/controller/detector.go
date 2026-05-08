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
	"fmt"
	v1alpha1 "github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"sort"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Generator struct {
	client          client.Client
	versionMu       sync.Mutex
	versionCounter  int64
	peerResolver    PeerResolver
	policyEvaluator PolicyEvaluator
}

// PeerStateSnapshot
type PeerStateSnapshot struct {
	Peer     *v1alpha1.LatticePeer
	Network  *v1alpha1.LatticeNetwork
	Policies []*v1alpha1.LatticePolicy
	Peers    []*v1alpha1.LatticePeer
	Labels   map[string]string
}

func NewGenerator(client client.Client) *Generator {
	return &Generator{
		client:          client,
		peerResolver:    NewPeerResolver(),
		policyEvaluator: NewPolicyEvaluator(),
	}
}

// DetectNodeChanges detects all changes to a Peer
//func (d *Generator) DetectNodeChanges(
//	ctx context.Context,
//	oldPeerSnapshot *PeerStateSnapshot,
//	oldPeer, newPeer *v1alpha1.LatticePeer,
//	oldNetwork, newNetwork *v1alpha1.LatticeNetwork,
//	oldPolicies, newPolicies []*v1alpha1.LatticePolicy,
//	req ctrl.Request,
//) *infra.DetailsInfo {
//
//	changes := &infra.DetailsInfo{
//		TotalChanges: 0,
//	}
//
//	//1. Detect node-level changes
//	d.detectNodeConfigChanges(ctx, changes, oldPeerSnapshot, oldPeer, newPeer, req)
//
//	// 2. Detect network topology changes (peers)
//	d.detectNetworkChanges(ctx, changes, oldPeerSnapshot, oldNetwork, newNetwork, req)
//
//	//3. Detect network policy changes
//	d.detectPolicyChanges(ctx, changes, oldPolicies, newPolicies, req)
//
//	return changes
//}
//
//func (d *Generator) detectNodeConfigChanges(ctx context.Context, changes *infra.DetailsInfo, oldNodeCtx *PeerStateSnapshot, oldNode, newNode *v1alpha1.LatticePeer, req ctrl.Request) *infra.DetailsInfo {
//	var newCreated bool
//	if oldNode == nil {
//		newCreated = true
//	}
//	// 1. Detect node's own changes
//	if !newCreated {
//		// IP address change
//		if !reflect.DeepEqual(oldNode.Status.AllocatedAddress, newNode.Status.AllocatedAddress) {
//			changes.AddressChanged = true
//			changes.TotalChanges++
//			changes.Reason = append(changes.Reason, &infra.Entry{
//				"AddressChanges", "address", "address changed",
//			})
//		}
//
//		// Key change
//		if oldNode.Spec.PublicKey != newNode.Spec.PublicKey {
//			changes.KeyChanged = true
//			changes.TotalChanges++
//			changes.Reason = append(changes.Reason, &infra.Entry{
//				"key changes", "address", "key changed",
//			})
//		}
//
//		if oldNode.Spec.PrivateKey != newNode.Spec.PrivateKey {
//			changes.KeyChanged = true
//			changes.TotalChanges++
//		}
//
//		// Network membership change
//		oldNet, newNet := oldNode.Spec.Network, newNode.Spec.Network
//		if newNet != nil {
//			if oldNet == nil {
//				changes.NetworkJoined = append(changes.NetworkJoined, *newNet)
//				changes.TotalChanges++
//				changes.Reason = append(changes.Reason, &infra.Entry{
//					"Network Addeed", "NetworkAdd", "Network new created",
//				})
//			}
//
//			if oldNet != nil && *newNet != *oldNet {
//				changes.NetworkLeft = append(changes.NetworkLeft, *oldNet)
//				changes.TotalChanges++
//				changes.Reason = append(changes.Reason, &infra.Entry{
//					"Leave Network", "NetworkLeave", "Network removed",
//				})
//			}
//		}
//
//		// Labels change
//		oldLables, newLables := oldNode.Labels, newNode.Labels
//		if !reflect.DeepEqual(oldLables, newLables) {
//			changes.TotalChanges++
//			changes.Reason = append(changes.Reason, &infra.Entry{
//				"Label Changed", "labels", "labels changed",
//			})
//		}
//
//		return changes
//	}
//
//	// New node
//	changes.TotalChanges++
//
//	return changes
//}
//
//func (d *Generator) detectNetworkChanges(ctx context.Context, changes *infra.DetailsInfo, oldNodeCtx *PeerStateSnapshot, oldNetwork, newNetwork *v1alpha1.LatticeNetwork, req ctrl.Request) *infra.DetailsInfo {
//	networkUpdateType := d.detectNetworkUpdateType(oldNetwork, newNetwork)
//
//	switch networkUpdateType {
//	case typeAdd:
//		changes.NetworkJoined = []string{newNetwork.Name}
//		peers, err := d.findNodes(ctx, newNetwork.Name)
//		if err != nil {
//			return changes
//		}
//		changes.PeersAdded = peers
//		changes.TotalChanges++
//		changes.Reason = append(changes.Reason, &infra.Entry{
//			"NetworkJoined", "NetworkJoined", "network joined",
//		})
//		return changes
//	case typeDel:
//		changes.NetworkLeft = []string{oldNetwork.Name}
//		changes.Reason = append(changes.Reason, &infra.Entry{
//			"NetworkLeft", "NetworkLeft", "network changed",
//		})
//		peers, err := d.findNodes(ctx, oldNetwork.Name)
//		if err != nil {
//			return changes
//		}
//		changes.PeersRemoved = peers
//		changes.TotalChanges++
//		return changes
//	}
//
//	return changes
//}

type changeType int // nolint

// nolint:all
const (
	typeNone changeType = iota
	typeAdd
	typeDel
	//typeUpdate
)

//
//func (d *Generator) detectNetworkUpdateType(oldNetwork, newNetwork *v1alpha1.LatticeNetwork) changeType {
//	if oldNetwork == nil && newNetwork == nil {
//		return typeNone
//	}
//	if oldNetwork == nil && newNetwork != nil {
//		return typeAdd
//	}
//
//	if oldNetwork != nil && newNetwork == nil {
//		return typeDel
//	}
//
//	return typeNone
//}
//
//func (d *Generator) detectPolicyUpdateType(oldPolicies, newPolicies []*v1alpha1.LatticePolicy) changeType {
//
//	if oldPolicies == nil && newPolicies != nil {
//		return typeAdd
//	}
//
//	if oldPolicies != nil && newPolicies == nil {
//		return typeDel
//	}
//
//	return typeNone
//}
//
//func (d *Generator) detectPolicyChanges(ctx context.Context, changes *infra.DetailsInfo, oldPolicies, newPolicies []*v1alpha1.LatticePolicy, req ctrl.Request) *infra.DetailsInfo {
//
//	policyUpdateType := d.detectPolicyUpdateType(oldPolicies, newPolicies)
//
//	switch policyUpdateType {
//	case typeAdd:
//		changes.Reason = append(changes.Reason, &infra.Entry{
//			"PolicyUpdate", "PolicyUpdate", "Policy added",
//		})
//		policies := make([]*infra.Policy, 0)
//		for _, p := range newPolicies {
//			policies = append(policies, d.buildPolicy(ctx, p))
//		}
//		changes.PoliciesAdded = append(changes.PoliciesAdded, policies...)
//		changes.TotalChanges++
//		return changes
//	case typeDel:
//		changes.Reason = append(changes.Reason, &infra.Entry{
//			"PolicyDeleted", "PolicyDeleted", "Policy removed",
//		})
//		policies := make([]*infra.Policy, 0)
//		for _, p := range oldPolicies {
//			policies = append(policies, d.buildPolicy(ctx, p))
//		}
//
//		changes.PoliciesRemoved = append(changes.PoliciesRemoved, policies...)
//		changes.TotalChanges++
//		return changes
//	}
//
//	return changes
//}

//func (d *Generator) findPolicy(ctx context.Context, node *v1alpha1.LatticePeer, req ctrl.Request) ([]*infra.Policy, error) {
//	var policyList v1alpha1.LatticePolicyList
//	if err := d.client.List(ctx, &policyList, client.InNamespace(req.Namespace)); err != nil {
//		return nil, err
//	}
//
//	var policies []*infra.Policy
//
//	for _, policy := range policyList.Items {
//		selector, _ := metav1.LabelSelectorAsSelector(&policy.Spec.PeerSelector)
//		matched := selector.Matches(labels.Set(node.Labels))
//		if matched {
//			p := d.buildPolicy(ctx, &policy)
//			policies = append(policies, p)
//		}
//	}
//
//	return policies, nil
//}

//func (d *Generator) findNodes(ctx context.Context, networkName string) ([]*infra.Peer, error) {
//	log := logf.FromContext(ctx)
//	log.Info("findNodes by network labels", "networkName", networkName)
//
//	labels := map[string]string{
//		"alattice.io/network-": networkName,
//	}
//
//	var peers v1alpha1.LatticePeerList
//	if err := d.client.List(ctx, &peers, client.MatchingLabels(labels)); err != nil {
//		return nil, err
//	}
//
//	var addedPeers []*infra.Peer
//	for _, item := range peers.Items {
//		addedPeers = append(addedPeers, transferToPeer(&item))
//	}
//
//	return addedPeers, nil
//}

func (d *Generator) generate(ctx context.Context, current *v1alpha1.LatticePeer, snapshot *PeerStateSnapshot, version string) (*infra.Message, error) {
	var err error
	// Generate config version number
	msg := &infra.Message{
		EventType:     infra.EventTypeNodeUpdate, // consistently use ConfigUpdate
		ConfigVersion: version,
		Timestamp:     time.Now().Unix(),
		Current:       transferToPeer(current),
		Network: &infra.Network{
			Peers: make([]*infra.Peer, 0),
		},
	}

	msg.Current.Labels = snapshot.Labels

	// Populate network info
	if snapshot.Network != nil {
		msg.Network.NetworkId = snapshot.Network.Name
		msg.Network.NetworkName = snapshot.Network.Spec.Name

		// Populate peers, sort by Name for stable hashing
		for _, p := range snapshot.Peers {
			if p.Status.AllocatedAddress == nil {
				continue
			}

			msg.Network.Peers = append(msg.Network.Peers, transferToPeer(p))
		}
		sort.Slice(msg.Network.Peers, func(i, j int) bool {
			return msg.Network.Peers[i].Name < msg.Network.Peers[j].Name
		})
	}

	msg.ComputedPeers, err = d.peerResolver.ResolvePeers(ctx, msg, snapshot.Policies)
	if err != nil {
		return nil, err
	}

	if snapshot.Policies != nil {
		msg.Policies = make([]*infra.Policy, 0)
		for _, p := range snapshot.Policies {
			msg.Policies = append(msg.Policies, d.buildPolicy(ctx, p))
		}
	}

	msg.ComputedRules, err = d.policyEvaluator.Evaluate(ctx, msg.Current, msg.Network, msg.Policies)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

//func peerToSet(peers []*infra.Peer) map[string]*infra.Peer {
//	m := make(map[string]*infra.Peer)
//	for _, peer := range peers {
//		m[peer.Name] = peer
//	}
//
//	return m
//}

// generateConfigVersion generates a config version string
func (d *Generator) generateConfigVersion() string {
	d.versionMu.Lock()
	defer d.versionMu.Unlock()

	d.versionCounter++
	return fmt.Sprintf("v%d", d.versionCounter)
}

func (d *Generator) buildPolicy(ctx context.Context, src *v1alpha1.LatticePolicy) *infra.Policy {
	log := logf.FromContext(ctx)
	log.Info("buildPolicy", "policy", src.Name)

	action := src.Spec.Action
	if action == "" {
		action = "ALLOW"
	}

	policy := &infra.Policy{
		PolicyName: src.Name,
		Action:     action,
	}

	var ingresses, egresses []*infra.Rule
	for _, ingress := range src.Spec.Ingress {
		names, err := d.getPeerNamesByLabels(ctx, src.Namespace, src.Spec.Network, ingress.From)
		if err != nil {
			log.Error(err, "failed to get peers from labels", "labels", ingress.From)
			continue
		}
		cidrs := extractCIDRs(ingress.From)

		if len(ingress.Ports) == 0 {
			ingresses = append(ingresses, &infra.Rule{
				PeerNames: names,
				CIDRs:     cidrs,
				Action:    action,
			})
		} else {
			for _, port := range ingress.Ports {
				ingresses = append(ingresses, &infra.Rule{
					PeerNames: names,
					CIDRs:     cidrs,
					Protocol:  port.Protocol,
					Port:      int(port.Port),
					Action:    action,
				})
			}
		}
	}

	for _, egress := range src.Spec.Egress {
		names, err := d.getPeerNamesByLabels(ctx, src.Namespace, src.Spec.Network, egress.To)
		if err != nil {
			log.Error(err, "failed to get peers from labels", "labels", egress.To)
			continue
		}
		cidrs := extractCIDRs(egress.To)

		if len(egress.Ports) == 0 {
			egresses = append(egresses, &infra.Rule{
				PeerNames: names,
				CIDRs:     cidrs,
				Action:    action,
			})
		} else {
			for _, port := range egress.Ports {
				egresses = append(egresses, &infra.Rule{
					PeerNames: names,
					CIDRs:     cidrs,
					Protocol:  port.Protocol,
					Port:      int(port.Port),
					Action:    action,
				})
			}
		}
	}

	policy.Ingress = ingresses
	policy.Egress = egresses

	return policy
}

// getPeerNamesByLabels finds peers by label selector, returning only the name list.
// Uses UID for deduplication to avoid duplicates when multiple selectors match the same peer; sorts by Name for stable hashing.
func (d *Generator) getPeerNamesByLabels(ctx context.Context, namespace, networkName string, rules []v1alpha1.PeerSelection) ([]string, error) {
	found := make(map[types.UID]string) // UID → Name

	for _, rule := range rules {
		if rule.PeerSelector == nil {
			continue // IPBlock entries handled separately
		}
		selector, err := metav1.LabelSelectorAsSelector(rule.PeerSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label selector %v: %w", rule.PeerSelector, err)
		}

		listOpts := []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabelsSelector{Selector: selector},
		}
		if networkName != "" {
			listOpts = append(listOpts, client.MatchingLabels{
				networkLabelKey(networkName): "true",
			})
		}

		var nodeList v1alpha1.LatticePeerList
		if err := d.client.List(ctx, &nodeList, listOpts...); err != nil {
			return nil, fmt.Errorf("failed to list nodes with selector %s: %w", selector.String(), err)
		}

		for _, node := range nodeList.Items {
			found[node.UID] = node.Name
		}
	}

	names := make([]string, 0, len(found))
	for _, name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// extractCIDRs collects CIDR strings from IPBlock entries in the selection list.
func extractCIDRs(rules []v1alpha1.PeerSelection) []string {
	var cidrs []string
	for _, r := range rules {
		if r.IPBlock != nil && r.IPBlock.CIDR != "" {
			cidrs = append(cidrs, r.IPBlock.CIDR)
		}
	}
	return cidrs
}
