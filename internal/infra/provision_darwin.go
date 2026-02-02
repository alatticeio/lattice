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
	"os"
	"os/exec"
	"strings"
)

func (r *routeProvisioner) ApplyRoute(action, address, interfaceName string) error {
	//example: sudo route -nv add -net 192.168.10.1 -netmask 255.255.255.0 -interface en0
	switch action {
	case "add":
		//ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s %s %s", interfaceName, address, address))
		rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		if err := ExecCommand("/bin/sh", "-c", rule); err != nil {
			return err
		}
		r.logger.Debug("root command issued", "cmd", fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName))
	case "delete":
		rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		if err := ExecCommand("/bin/sh", "-c", rule); err != nil {
			return err
		}
		r.logger.Debug("root command command", "cmd", fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName))
	}

	return nil
}

func (r *routeProvisioner) ApplyIP(action, address, name string) error {
	switch action {
	case "add":
		if err := ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s %s %s", name, address, address)); err != nil {
			return err
		}

	}

	return nil
}

func (r *ruleProvisioner) Name() string { return "pfctl" }

func (r *ruleProvisioner) Provision(rule *FirewallRule) error {
	var sb strings.Builder
	anchor := "wireflow"

	// 1. 生成 PF 规则字符串
	// Ingress: pass in proto tcp from {IP1, IP2} to any port 80
	for _, tr := range rule.Ingress {
		ips := "{" + strings.Join(tr.Peers, ", ") + "}"
		sb.WriteString(fmt.Sprintf("pass in proto %s from %s to any port %d\n",
			strings.ToLower(tr.Protocol), ips, tr.Port))
	}

	// Egress: pass out proto tcp from any to {IP1} port 3306
	for _, tr := range rule.Egress {
		ips := "{" + strings.Join(tr.Peers, ", ") + "}"
		sb.WriteString(fmt.Sprintf("pass out proto %s from any to %s port %d\n",
			strings.ToLower(tr.Protocol), ips, tr.Port))
	}

	// 2. 默认拒绝 (零信任封口)
	// 注意：macOS PF 默认是放行的，需显式加入 block
	sb.WriteString("block in on utun4 all\n")
	sb.WriteString("block out on utun4 all\n")

	// 3. 将规则写入临时文件并加载到 anchor
	tmpFile := "/tmp/wireflow.pf"
	os.WriteFile(tmpFile, []byte(sb.String()), 0644)

	// 使用 pfctl 加载特定的 anchor，不影响系统其他规则
	cmd := exec.Command("sudo", "pfctl", "-a", anchor, "-f", tmpFile)
	return cmd.Run()
}

func (p *ruleProvisioner) Cleanup() error {
	return exec.Command("sudo", "pfctl", "-a", "wireflow", "-F", "all").Run()
}

func (r *ruleProvisioner) SetupNAT(interfaceName string) error {
	return nil
}
