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

	// [Step 1] 构建当前网络内所有 Peer 的 IP 集合。
	// getPeerFromLabels 会跨全集群查询，这里用网络内的 IP 做二次过滤，
	// 确保只有同一网络内的 Peer IP 才会写入防火墙规则。
	networkPeerIPs := make(map[string]struct{}, len(network.Peers))
	for _, p := range network.Peers {
		if p.Address != nil {
			networkPeerIPs[cleanIP(p.Address)] = struct{}{}
		}
	}

	// [Step 2] 处理 Ingress 规则 (INPUT 链：别人 -> 我)
	for _, policy := range allPolicies {
		result.PolicyName = policy.PolicyName // 最后一条策略名；多策略时作日志用途
		for _, rule := range policy.Ingress {
			for _, sourcePeer := range rule.Peers {
				if sourcePeer.Address == nil || sourcePeer.Name == currentPeer.Name {
					continue
				}
				srcIP := cleanIP(sourcePeer.Address)
				if srcIP == "" {
					continue
				}
				// 只允许同一网络内的 Peer，过滤掉来自其他网络的 IP
				if _, inNetwork := networkPeerIPs[srcIP]; !inNetwork {
					log.Info("Skipping peer not in current network", "peer", sourcePeer.Name, "ip", srcIP)
					continue
				}
				trafficRule := infra.TrafficRule{
					ChainName: "WIREFLOW-INGRESS",
					Peers:     []string{srcIP},
					Port:      rule.Port,
					Protocol:  rule.Protocol,
					Action:    "ACCEPT",
				}
				result.Ingress = append(result.Ingress, trafficRule)
			}
		}
	}

	// [Step 3] 处理 Egress 规则 (OUTPUT 链：我 -> 别人)
	for _, policy := range allPolicies {
		for _, rule := range policy.Egress {
			for _, destPeer := range rule.Peers {
				if destPeer.Address == nil || destPeer.Name == currentPeer.Name {
					continue
				}
				destIP := cleanIP(destPeer.Address)
				if destIP == "" {
					continue
				}
				if _, inNetwork := networkPeerIPs[destIP]; !inNetwork {
					log.Info("Skipping peer not in current network", "peer", destPeer.Name, "ip", destIP)
					continue
				}
				trafficRule := infra.TrafficRule{
					ChainName: "WIREFLOW-EGRESS",
					Peers:     []string{destIP},
					Port:      rule.Port,
					Protocol:  rule.Protocol,
					Action:    "ACCEPT",
				}
				result.Egress = append(result.Egress, trafficRule)
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
