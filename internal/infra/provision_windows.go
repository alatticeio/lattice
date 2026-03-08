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

package infra

import (
	"fmt"
	"os/exec"
	"strings"
)

func (r *routeProvisioner) ApplyRoute(action, address, interfaceName string) error {
	ip := TrimCIDR(address)
	gateway := GetGatewayFromIP(ip)

	// Use the route command for Windows, add or delete route
	ExecCommand("cmd", "/C", fmt.Sprintf(
		"route %s %s mask 255.255.255.0 %s", action, ip, gateway))
	return nil
}

func (r *routeProvisioner) ApplyIP(action, address, name string) error {
	switch action {
	case "add":
		ip := TrimCIDR(address)
		// Set the IP address using netsh on Windows
		ExecCommand("cmd", "/C", fmt.Sprintf(
			"netsh interface ipv4 set address name=\"%s\" static %s 255.255.255.0",
			name, ip))

		// Enable the network interface
		ExecCommand("cmd", "/C", fmt.Sprintf(
			"netsh interface set interface name=\"%s\" admin=ENABLED", name))
	}
	return nil
}

func (r *ruleProvisioner) Name() string {
	return "windows-fw"
}

func (r *ruleProvisioner) Provision(rule *FirewallRule) error {
	// 1. 清理旧规则 (基于 Name 前缀)
	r.execPS("Remove-NetFirewallRule -DisplayName 'Wireflow-*'")

	// 2. 处理 Ingress
	for i, tr := range rule.Ingress {
		ips := strings.Join(tr.Peers, ",")
		cmd := fmt.Sprintf(
			"New-NetFirewallRule -DisplayName 'Wireflow-In-%d' -Direction Inbound -Action Allow -Protocol %s -LocalPort %d -RemoteAddress %s",
			i, strings.ToUpper(tr.Protocol), tr.Port, ips,
		)
		if err := r.execPS(cmd); err != nil {
			return err
		}
	}

	// 3. 处理 Egress
	for i, tr := range rule.Egress {
		ips := strings.Join(tr.Peers, ",")
		cmd := fmt.Sprintf(
			"New-NetFirewallRule -DisplayName 'Wireflow-Out-%d' -Direction Outbound -Action Allow -Protocol %s -RemotePort %d -RemoteAddress %s",
			i, strings.ToUpper(tr.Protocol), tr.Port, ips,
		)
		if err := r.execPS(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *ruleProvisioner) execPS(command string) error {
	cmd := exec.Command("powershell", "-Command", command)
	return cmd.Run()
}

func (p *ruleProvisioner) Cleanup() error {
	return nil
}

func (r *ruleProvisioner) SetupNAT(interfaceName string) error {
	// Example of enabling NAT through RRAS, assuming RRAS is configured
	// Windows doesn't directly support iptables-like NAT, but you can use
	// Windows Routing and Remote Access Service (RRAS) for NAT configuration
	cmd := fmt.Sprintf(
		"netsh routing ip nat add interface %s", interfaceName)
	return r.execPS(cmd)
}
