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
	"strings"
	"wireflow/internal/infra"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// 关注L3/L4的策略，根据Policy中的ingress来实现
// Default deny， 只生成Accept策略， 零信任
type FirewallRuleResolver interface {
	ResolveRules(ctx context.Context, currentPeer *infra.Peer, network *infra.Network, policies []*infra.Policy) (*infra.FirewallRule, error)
}

type firewallRuleResolver struct {
}

func NewFirewallResolver() FirewallRuleResolver {
	return &firewallRuleResolver{}
}

func (r *firewallRuleResolver) ResolveRules(ctx context.Context, currentPeer *infra.Peer, network *infra.Network, allPolicies []*infra.Policy) (*infra.FirewallRule, error) {
	log := logf.FromContext(ctx)
	log.Info("Resolving firewall rules", "currentPeer", currentPeer, "network", network)
	if currentPeer == nil || network == nil {
		return nil, fmt.Errorf("currentPeer or network cannot be nil")
	}

	result := &infra.FirewallRule{
		Platform: currentPeer.Platform,
		Ingress:  make([]infra.TrafficRule, 0),
		Egress:   make([]infra.TrafficRule, 0),
	}

	// [Step 1] 筛选出适用于当前 Peer 的策略
	appliedPolicies := allPolicies

	// [Step 2] 处理 Ingress 规则 (INPUT 链：别人 -> 我)
	for _, policy := range appliedPolicies {
		for _, rule := range policy.Ingress {
			for _, sourcePeer := range rule.Peers {
				if sourcePeer.Address == nil || sourcePeer.Name == currentPeer.Name {
					continue
				}

				srcIP := cleanIP(sourcePeer.Address)
				trafficRule := infra.TrafficRule{
					ChainName: "WIREFLOW-INGRESS",
					Peers:     make([]string, 0),
					Port:      rule.Port,
					Protocol:  rule.Protocol,
					Action:    "ACCEPT",
				}
				trafficRule.Peers = append(trafficRule.Peers, srcIP)
				result.Ingress = append(result.Ingress, trafficRule)
			}
		}
	}

	// [Step 3] 处理 Egress 规则 (OUTPUT 链：我 -> 别人)
	for _, policy := range appliedPolicies {
		for _, rule := range policy.Egress {
			for _, destPeer := range rule.Peers {
				if destPeer.Address == nil || destPeer.Name == currentPeer.Name {
					continue
				}

				destIP := cleanIP(destPeer.Address)
				trafficRule := infra.TrafficRule{
					ChainName: "WIREFLOW-EGRESS",
					Peers:     make([]string, 0),
					Port:      rule.Port,
					Protocol:  rule.Protocol,
					Action:    "ACCEPT",
				}
				trafficRule.Peers = append(trafficRule.Peers, destIP)
				result.Egress = append(result.Egress, trafficRule)

			}
		}
	}

	return result, nil
}

// cleanIP 辅助函数：去除 CIDR 后缀 (例如 "10.0.0.1/32" -> "10.0.0.1")
func cleanIP(ip *string) string {
	if ip != nil {
		if strings.Contains(*ip, "/") {
			return strings.Split(*ip, "/")[0]
		}
	}
	return ""
}
