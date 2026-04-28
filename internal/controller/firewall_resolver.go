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

	// [Step 1] 用 network.Peers 构建 name→IP 查找表。
	// Rule 现在只存 peer name，需要在这里解析出实际 IP。
	// 同时起到网络隔离作用：不在 network.Peers 里的 name 查不到 IP，自然过滤。
	peerIPByName := make(map[string]string, len(network.Peers))
	for _, p := range network.Peers {
		if p.Address != nil {
			peerIPByName[p.Name] = cleanIP(p.Address)
		}
	}

	// [Step 2] 处理 Ingress 规则 (INPUT 链：别人 -> 我)
	for _, policy := range allPolicies {
		result.PolicyName = policy.PolicyName // 最后一条策略名；多策略时作日志用途
		for _, rule := range policy.Ingress {
			for _, peerName := range rule.PeerNames {
				if peerName == currentPeer.Name {
					continue
				}
				srcIP := peerIPByName[peerName]
				if srcIP == "" {
					log.Info("Skipping peer not in current network or no IP yet", "peer", peerName)
					continue
				}
				result.Ingress = append(result.Ingress, infra.TrafficRule{
					ChainName: "WIREFLOW-INGRESS",
					Peers:     []string{srcIP},
					Port:      rule.Port,
					Protocol:  rule.Protocol,
					Action:    "ACCEPT",
				})
			}
		}
	}

	// [Step 3] 处理 Egress 规则 (OUTPUT 链：我 -> 别人)
	for _, policy := range allPolicies {
		for _, rule := range policy.Egress {
			for _, peerName := range rule.PeerNames {
				if peerName == currentPeer.Name {
					continue
				}
				destIP := peerIPByName[peerName]
				if destIP == "" {
					log.Info("Skipping peer not in current network or no IP yet", "peer", peerName)
					continue
				}
				result.Egress = append(result.Egress, infra.TrafficRule{
					ChainName: "WIREFLOW-EGRESS",
					Peers:     []string{destIP},
					Port:      rule.Port,
					Protocol:  rule.Protocol,
					Action:    "ACCEPT",
				})
			}
		}
	}

	return result, nil
}

// cleanIP 辅助函数：去除 CIDR 后缀 (例如 "10.0.0.1/32" -> "10.0.0.1")
// 若不含 CIDR 后缀则原样返回；若 ip 为 nil 则返回空字符串。
func cleanIP(ip *string) string {
	if ip == nil {
		return ""
	}
	if strings.Contains(*ip, "/") {
		return strings.Split(*ip, "/")[0]
	}
	return *ip
}
