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

package provision

import (
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var pfMu sync.Mutex

func (r *routeProvisioner) ApplyRoute(action, address, interfaceName string) error {
	//example: sudo route -nv add -net 192.168.10.1 -netmask 255.255.255.0 -interface en0
	switch action {
	case "add":
		//infra.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s %s %s", interfaceName, address, address))
		rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		if err := infra.ExecCommand("/bin/sh", "-c", rule); err != nil {
			return err
		}
		r.logger.Debug("root command issued", "cmd", fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName))
	case "delete":
		rule := fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName)
		if err := infra.ExecCommand("/bin/sh", "-c", rule); err != nil {
			return err
		}
		r.logger.Debug("root command command", "cmd", fmt.Sprintf("route -nv %s -net %s -netmask 255.255.255.0 -interface %s", action, address, interfaceName))
	}

	return nil
}

func (r *routeProvisioner) ApplyIP(action, address, name string) error {
	switch action {
	case "add":
		if err := infra.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s %s %s", name, address, address)); err != nil {
			return err
		}
		if err := infra.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ifconfig %s mtu %d", name, infra.DefaultMTU)); err != nil {
			return err
		}

	}

	return nil
}

func (r *ruleProvisioner) Name() string { return "pfctl" }

func ensurePFReady(anchor string) error {
	// 确保 pf 已启用
	exec.Command("sudo", "pfctl", "-e").Run() //nolint:errcheck

	// 检查 anchor 是否已在主 ruleset 中
	out, _ := exec.Command("sudo", "pfctl", "-sr").Output()
	if strings.Contains(string(out), `anchor "`+anchor+`"`) {
		return nil
	}

	// 将 anchor 追加到主 ruleset 并重载
	existing := strings.TrimRight(string(out), "\n")
	merged := existing + fmt.Sprintf("\nanchor \"%s\"\n", anchor)
	cmd := exec.Command("sudo", "pfctl", "-f", "-")
	cmd.Stdin = strings.NewReader(merged)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to register pf anchor: %w\n%s", err, output)
	}
	return nil
}

func (r *ruleProvisioner) Provision(rule *infra.FirewallRule) error {
	pfMu.Lock()
	defer pfMu.Unlock()

	var sb strings.Builder
	anchor := "lattice"

	if err := ensurePFReady(anchor); err != nil {
		return err
	}

	// 1. 默认拒绝 (零信任封口)
	// 注意：PF 是 last-match-wins，block 必须写在 pass 之前作为兜底
	iface := r.interfaceName
	fmt.Fprintf(&sb, "block in on %s all\n", iface)
	fmt.Fprintf(&sb, "block out on %s all\n", iface)

	// 2. 生成 PF 规则字符串
	// 当 Protocol 或 Port 未指定（零值）时，省略 proto/port，允许该 IP 的所有流量。
	// Ingress: pass in [proto tcp] from {IP1} [to any port 80]
	for _, tr := range rule.Ingress {
		ips := "{" + strings.Join(tr.Peers, ", ") + "}"
		if tr.Protocol != "" && tr.Port != 0 {
			fmt.Fprintf(&sb, "pass in proto %s from %s to any port %d\n",
				strings.ToLower(tr.Protocol), ips, tr.Port)
		} else {
			fmt.Fprintf(&sb, "pass in from %s\n", ips)
		}
	}

	// Egress: pass out [proto tcp] from any to {IP1} [port 3306]
	for _, tr := range rule.Egress {
		ips := "{" + strings.Join(tr.Peers, ", ") + "}"
		if tr.Protocol != "" && tr.Port != 0 {
			fmt.Fprintf(&sb, "pass out proto %s from any to %s port %d\n",
				strings.ToLower(tr.Protocol), ips, tr.Port)
		} else {
			fmt.Fprintf(&sb, "pass out to %s\n", ips)
		}
	}

	// 3. 将规则写入临时文件并加载到 anchor
	tmpFile := "/tmp/lattice.pf"
	if err := os.WriteFile(tmpFile, []byte(sb.String()), 0644); err != nil {
		return err
	}

	// 使用 pfctl 加载特定的 anchor，不影响系统其他规则
	cmd := exec.Command("sudo", "pfctl", "-a", anchor, "-f", tmpFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pfctl command failed: %w\n%s", err, output)
	}
	return nil
}

func (p *ruleProvisioner) Cleanup() error {
	return exec.Command("sudo", "pfctl", "-a", "lattice", "-F", "all").Run()
}

func (r *ruleProvisioner) SetupNAT(interfaceName string) error {
	return nil
}
