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
	"wireflow/internal/core/domain"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// 关注L3/L4的策略，根据Policy中的ingress来实现
// Default deny， 只生成Accept策略， 零信任
type FirewallRuleResolver interface {
	ResolveRules(ctx context.Context, currentPeer *domain.Peer, network *domain.Network, policies []*domain.Policy) (*domain.FirewallRule, error)
}

type firewallRuleResolver struct {
	ruleGenerator RuleGenerator
}

func NewFirewallResolver() FirewallRuleResolver {
	return &firewallRuleResolver{}
}

func (r *firewallRuleResolver) ResolveRules(ctx context.Context, currentPeer *domain.Peer, network *domain.Network, allPolicies []*domain.Policy) (*domain.FirewallRule, error) {
	log := logf.FromContext(ctx)
	log.Info("Resolving firewall rules")
	if currentPeer == nil || network == nil {
		return nil, fmt.Errorf("currentPeer or network cannot be nil")
	}

	result := &domain.FirewallRule{
		Platform:     currentPeer.Platform,
		IngressRules: make([]string, 0),
		EgressRules:  make([]string, 0),
	}

	generator, err := NewRuleGenerator(currentPeer.Platform)
	if err != nil {
		log.Error(err, "Error generating firewall rule generator")
		return nil, err
	}

	// [Step 0] 状态检测规则 (Stateful Inspection) - 必须放在最前面
	// 允许已建立连接的流量通过（RELATED,ESTABLISHED）。
	statefulRule := fmt.Sprintf("-m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT")
	result.IngressRules = append(result.IngressRules, fmt.Sprintf("-A INPUT -i %s %s", currentPeer.InterfaceName, statefulRule))
	result.EgressRules = append(result.EgressRules, fmt.Sprintf("-A OUTPUT -o %s %s", currentPeer.InterfaceName, statefulRule))

	// [Step 1] 筛选出适用于当前 Peer 的策略
	appliedPolicies := allPolicies
	//appliedPolicies := make([]*domain.Policy, 0)
	//for _, policy := range allPolicies {
	//	for _, appliedID := range policy.AppliedToPeerIDs {
	//		if appliedID == currentPeer.Name {
	//			appliedPolicies = append(appliedPolicies, policy)
	//			break
	//		}
	//	}
	//}

	// [Step 2] 处理 Ingress 规则 (INPUT 链：别人 -> 我)
	for _, policy := range appliedPolicies {
		for _, rule := range policy.Ingress {
			for _, sourcePeer := range rule.Peers {
				if sourcePeer.Address == nil || sourcePeer.Name == currentPeer.Name {
					continue
				}

				srcIP := cleanIP(sourcePeer.Address)
				baseCmd := fmt.Sprintf("-s %s", srcIP)

				// 命令格式: -A INPUT -i wg0 -s <IP> ... -j ACCEPT
				// 调用平台策略生成最终命令
				fullCmd, err := generator.GenerateRule("INPUT", baseCmd, rule, sourcePeer)
				if err != nil {
					return nil, err
				}
				result.IngressRules = append(result.IngressRules, fullCmd)

				result.IngressRules = append(result.IngressRules, fullCmd)
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

				// 命令格式: -A OUTPUT -o wg0 -d <IP> ... -j ACCEPT
				baseCmd := fmt.Sprintf("-A OUTPUT -o %s -d %s", currentPeer.InterfaceName, destIP)
				// 调用平台策略生成最终命令
				fullCmd, err := generator.GenerateRule("OUTPUT", baseCmd, rule, destPeer)
				if err != nil {
					return nil, err
				}
				result.EgressRules = append(result.EgressRules, fullCmd)

			}
		}
	}

	// [Step 4] 默认拒绝 (Default Deny) - 兜底规则
	// 放在链的最后，丢弃所有未匹配上述 ACCEPT 规则的流量
	result.IngressRules = append(result.IngressRules, fmt.Sprintf("-A INPUT -i %s -j DROP", currentPeer.InterfaceName))
	result.EgressRules = append(result.EgressRules, fmt.Sprintf("-A OUTPUT -o %s -j DROP", currentPeer.InterfaceName))

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
