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
	"strings"

	"github.com/alatticeio/lattice/internal/agent/infra"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// PolicyEvaluator resolves []*infra.Policy into a *infra.FirewallRule with
// ALLOW-priority conflict resolution and a default-deny tail rule.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, currentPeer *infra.Peer, network *infra.Network, policies []*infra.Policy) (*infra.FirewallRule, error)
}

// ruleDecisionKey uniquely identifies a traffic flow for deduplication.
type ruleDecisionKey struct {
	peer     string // IP or CIDR
	port     int
	protocol string
}

type policyEvaluator struct{}

// NewPolicyEvaluator returns a PolicyEvaluator with ALLOW-priority conflict resolution.
func NewPolicyEvaluator() PolicyEvaluator {
	return &policyEvaluator{}
}

func (e *policyEvaluator) Evaluate(ctx context.Context, currentPeer *infra.Peer, network *infra.Network, policies []*infra.Policy) (*infra.FirewallRule, error) {
	log := logf.FromContext(ctx)

	if currentPeer == nil || network == nil {
		return nil, fmt.Errorf("currentPeer and network must not be nil")
	}

	// Build name→IP lookup table from network peers.
	peerIPByName := make(map[string]string, len(network.Peers))
	for _, p := range network.Peers {
		if p.Address != nil {
			peerIPByName[p.Name] = cleanIP(p.Address)
		}
	}

	result := &infra.FirewallRule{
		Platform: currentPeer.Platform,
		Ingress:  make([]infra.TrafficRule, 0),
		Egress:   make([]infra.TrafficRule, 0),
	}

	// decisions maps (peer, port, protocol) → "ALLOW" | "DENY"
	// ALLOW is sticky: once set it cannot be overwritten.
	ingressDecisions := make(map[ruleDecisionKey]string)
	egressDecisions := make(map[ruleDecisionKey]string)

	applyDecision := func(decisions map[ruleDecisionKey]string, rule *infra.Rule) {
		peers := e.resolveRulePeers(rule, peerIPByName, currentPeer.Name)
		for _, peer := range peers {
			k := ruleDecisionKey{peer: peer, port: rule.Port, protocol: rule.Protocol}
			if decisions[k] != "ALLOW" {
				decisions[k] = rule.Action
			}
		}
	}

	for _, policy := range policies {
		if policy == nil {
			continue
		}
		for _, rule := range policy.Ingress {
			if rule == nil {
				continue
			}
			applyDecision(ingressDecisions, rule)
		}
		for _, rule := range policy.Egress {
			if rule == nil {
				continue
			}
			applyDecision(egressDecisions, rule)
		}
	}

	// Emit TrafficRules from decisions.
	for k, action := range ingressDecisions {
		iptAction := toIPTAction(action)
		tr := infra.TrafficRule{
			ChainName: "LATTICE-INGRESS",
			Peers:     []string{k.peer},
			Port:      k.port,
			Protocol:  k.protocol,
			Action:    iptAction,
		}
		result.Ingress = append(result.Ingress, tr)
	}

	for k, action := range egressDecisions {
		iptAction := toIPTAction(action)
		tr := infra.TrafficRule{
			ChainName: "LATTICE-EGRESS",
			Peers:     []string{k.peer},
			Port:      k.port,
			Protocol:  k.protocol,
			Action:    iptAction,
		}
		result.Egress = append(result.Egress, tr)
	}

	// Append default-deny tail rules (empty Peers = chain-tail DROP).
	result.Ingress = append(result.Ingress, infra.TrafficRule{
		ChainName: "LATTICE-INGRESS",
		Action:    "DROP",
	})
	result.Egress = append(result.Egress, infra.TrafficRule{
		ChainName: "LATTICE-EGRESS",
		Action:    "DROP",
	})

	log.Info("PolicyEvaluator done",
		"ingressRules", len(result.Ingress),
		"egressRules", len(result.Egress),
	)
	return result, nil
}

// resolveRulePeers returns the IP/CIDR list for a rule, skipping currentPeerName.
func (e *policyEvaluator) resolveRulePeers(rule *infra.Rule, peerIPByName map[string]string, currentPeerName string) []string {
	var peers []string
	for _, name := range rule.PeerNames {
		if name == currentPeerName {
			continue
		}
		ip := peerIPByName[name]
		if ip == "" {
			continue // peer not yet assigned an IP
		}
		peers = append(peers, ip)
	}
	for _, cidr := range rule.CIDRs {
		if strings.TrimSpace(cidr) != "" {
			peers = append(peers, cidr)
		}
	}
	return peers
}

// toIPTAction converts "ALLOW"/"DENY" to iptables target names.
func toIPTAction(action string) string {
	if strings.EqualFold(action, "ALLOW") {
		return "ACCEPT"
	}
	return "DROP"
}
