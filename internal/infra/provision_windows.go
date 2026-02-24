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
	//example: sudo route -nv add -net 192.168.10.1 -netmask 255.255.255.0 -interface en0
	// example: netsh interface ipv4 set address name="linkany-xx" static 192.168.1.10
	ip := TrimCIDR(address)
	gateway := GetGatewayFromIP(ip)

	ExecCommand("cmd", "/C", fmt.Sprintf(`route %s %s mask 255.255.255.0 %s`, action, ip, gateway))

	return nil
}

func (r *routeProvisioner) ApplyIP(action, address, name string) error {
	switch action {
	case "add":
		ip := TrimCIDR(address)
		ExecCommand("cmd", "/C", fmt.Sprintf(`netsh interface ipv4 set address name="%s" static %s 255.255.255.0`, name, ip))
		ExecCommand("cmd", "/C", fmt.Sprintf(`netsh interface set interface name="%s" admin=ENABLED`, name))
	}

	return nil
}

func (r *ruleProvisioner) Name() string { return "windows-fw" }

func (r *ruleProvisioner) Provision(rule *FirewallRule) error {
	// 1. 清理旧规则 (基于 Name 前缀)
	p.execPS("Remove-NetFirewallRule -DisplayName 'Wireflow-*'")

	// 2. 处理 Ingress
	for i, tr := range rule.Ingress {
		ips := strings.Join(tr.Peers, ",")
		cmd := fmt.Sprintf(
			"New-NetFirewallRule -DisplayName 'Wireflow-In-%d' -Direction Inbound -Action Allow -Protocol %s -LocalPort %d -RemoteAddress %s",
			i, strings.ToUpper(tr.Protocol), tr.Port, ips,
		)
		if err := p.execPS(cmd); err != nil {
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
		if err := p.execPS(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *WindowsProvisioner) execPS(command string) error {
	cmd := exec.Command("powershell", "-Command", command)
	return cmd.Run()
}

func (p *WindowsProvisioner) Cleanup() error {
	return p.execPS("Remove-NetFirewallRule -DisplayName 'Wireflow-*'")
}

func (r *ruleProvisioner) ApplyRule(action, rule string) error {
	return nil
}

func (r *ruleProvisioner) SetupNAT(interfaceName string) error {
	return nil
}
