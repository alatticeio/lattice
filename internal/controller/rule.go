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
	"fmt"
	"strings"
	"wireflow/internal/core/domain"
)

// RuleGenerator for mutiple platform generate firewall rules
type RuleGenerator interface {
	// Name 返回当前生成器所针对的平台名称
	Name() string

	// GenerateRule 将策略数据转换为平台特定的 ACCEPT 命令
	// chain: "INPUT" 或 "OUTPUT"
	// baseCmd: e.g., "-A INPUT -i wg0 -s 10.0.0.1" (通用部分)
	// peer: 规则涉及的对端 Peer
	GenerateRule(chain string, baseCmd string, rule *domain.Rule, peer *domain.Peer) (string, error)

	// GenerateStatefulAccept 生成状态检测规则（RELATED, ESTABLISHED）
	GenerateStatefulAccept(iface string, chain string) string

	// GenerateDefaultDeny 生成默认拒绝规则（DROP）
	GenerateDefaultDeny(iface string, chain string) string
}

// Platform 类型常量，用于避免字符串错误
const (
	PlatformLinux   = "linux"
	PlatformWindows = "windows"
	PlatformMacOS   = "macos"
	// 可以在此添加更多平台，如 FreeBSD, Android等
)

// NewRuleGenerator 是工厂函数，根据平台名称返回相应的 RuleGenerator 实例。
func NewRuleGenerator(platform string) (RuleGenerator, error) {
	// 将输入转换为小写，确保健壮性
	p := strings.ToLower(platform)

	switch p {
	case PlatformLinux:
		// 返回 Linux/iptables 的生成器实例
		return &IptablesGenerator{}, nil

	case PlatformWindows:
		// 返回 Windows/PowerShell 的生成器实例
		return &WindowsGenerator{}, nil

	case PlatformMacOS:
		// 如果您决定实现 macOS 的 pf/ipfw 生成器，可以在这里返回
		return nil, fmt.Errorf("platform '%s' is currently not supported (MacOSGenerator not implemented)", platform)

	default:
		return nil, fmt.Errorf("unsupported platform type: %s", platform)
	}
}

// WindowsGenerator 实现了 Windows 平台的 PowerShell 规则生成
type WindowsGenerator struct{}

func (g *WindowsGenerator) Name() string {
	return "Windows"
}

func (g *WindowsGenerator) GenerateRule(chain string, baseCmd string, rule *domain.Rule, peer *domain.Peer) (string, error) {
	direction := "Outbound"
	if chain == "INPUT" {
		direction = "Inbound"
	}

	// 命名规则：用于管理和删除
	ruleName := fmt.Sprintf("WG-%s-%s-%s-%s", direction, peer.Name, rule.Protocol, rule.Port)

	// PowerShell Cmdlet
	cmd := fmt.Sprintf("New-NetFirewallRule -DisplayName \"%s\" -Direction %s -Action Allow", ruleName, direction)

	// -RemoteAddress 相当于 iptables 的 -s 或 -d
	cmd += fmt.Sprintf(" -RemoteAddress %s", cleanIP(peer.Address))

	// 协议
	if rule.Protocol != "" && rule.Protocol != "all" {
		cmd += fmt.Sprintf(" -Protocol %s", strings.ToUpper(rule.Protocol))
	}

	// 端口：Windows 防火墙需明确区分 LocalPort (Ingress) 和 RemotePort (Egress)
	if rule.Port != "" && rule.Protocol != "icmp" {
		if direction == "Inbound" {
			cmd += fmt.Sprintf(" -LocalPort %s", rule.Port) // 规则应用于当前节点的端口
		} else {
			cmd += fmt.Sprintf(" -RemotePort %s", rule.Port) // 规则应用于目标 Peer 的端口
		}
	}

	// 限制接口 (可选，但推荐)
	cmd += fmt.Sprintf(" -InterfaceAlias \"%s\"", g.getInterfaceAlias(peer.Name)) // 假设 WireGuard 接口名是根据 Peer Name 生成的

	return cmd, nil
}

func (g *WindowsGenerator) GenerateStatefulAccept(iface string, chain string) string {
	// Windows 防火墙默认是状态跟踪的，通常无需此规则。
	return "# Windows: Stateful Inspection is enabled by default."
}

func (g *WindowsGenerator) GenerateDefaultDeny(iface string, chain string) string {
	// Windows 的 Default Deny 通常通过 Profile (Domain/Private/Public) 级别配置，
	// 不通过单个命名规则实现。
	return "# Windows: Default Deny policy is managed by the network profile configuration (e.g., Block all inbound connections)."
}

func (g *WindowsGenerator) getInterfaceAlias(peerName string) string {
	// 辅助函数：简化 WireGuard 接口名称查找
	return "WireGuard Tunnel" // 或使用更精确的别名
}

// IptablesGenerator 实现了 Linux 平台的 iptables 规则生成
type IptablesGenerator struct{}

func (g *IptablesGenerator) Name() string {
	return "Linux"
}

func (g *IptablesGenerator) GenerateRule(chain string, baseCmd string, rule *domain.Rule, peer *domain.Peer) (string, error) {
	// baseCmd: e.g., "-A INPUT -i wg0 -s 10.0.0.1"

	cmd := baseCmd

	// 1. 处理协议
	if rule.Protocol != "" && rule.Protocol != "all" {
		cmd += fmt.Sprintf(" -p %s", rule.Protocol)
	}

	// 2. 处理端口
	// iptables 中，目标端口使用 --dport
	if rule.Port != "" && rule.Protocol != "all" && rule.Protocol != "icmp" && rule.Protocol != "" {
		cmd += fmt.Sprintf(" --dport %s", rule.Port)
	}

	cmd += " -j ACCEPT"
	return cmd, nil
}

func (g *IptablesGenerator) GenerateStatefulAccept(iface string, chain string) string {
	// Linux 状态检测的标准命令
	return fmt.Sprintf("-A %s -i %s -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", chain, iface)
}

func (g *IptablesGenerator) GenerateDefaultDeny(iface string, chain string) string {
	// Linux 默认拒绝的标准命令
	return fmt.Sprintf("-A %s -i %s -j DROP", chain, iface)
}
