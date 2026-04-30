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
	"bytes"
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func (r *routeProvisioner) ApplyRoute(action, address, name string) error {
	cidr := infra.GetCidrFromIP(address)
	switch action {
	case "add":
		// Serialize under mu: iptables check→add is not atomic, and concurrent
		// callers (multiple onPeerKnown firing simultaneously) will race: both
		// see the rule absent, both attempt -A, the second gets xtables lock
		// error and returns exit status 1.  Holding the mutex makes the
		// check→add sequence atomic within this process.
		r.mu.Lock()
		iptCmds := fmt.Sprintf(
			"iptables -w 5 -C FORWARD -i %[1]s -j ACCEPT 2>/dev/null || iptables -w 5 -A FORWARD -i %[1]s -j ACCEPT; "+
				"iptables -w 5 -C FORWARD -o %[1]s -j ACCEPT 2>/dev/null || iptables -w 5 -A FORWARD -o %[1]s -j ACCEPT; "+
				"DEV=$(ip route show default | awk 'NR==1{print $5}'); "+
				"iptables -w 5 -t nat -C POSTROUTING -o \"$DEV\" -j MASQUERADE 2>/dev/null || iptables -w 5 -t nat -A POSTROUTING -o \"$DEV\" -j MASQUERADE",
			name,
		)
		iptErr := infra.ExecCommand("/bin/sh", "-c", iptCmds)
		r.mu.Unlock()
		if iptErr != nil {
			return iptErr
		}
		// ip route replace is idempotent; no lock needed.
		if err := infra.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip route replace %s dev %s", cidr, name)); err != nil {
			return err
		}
		r.logger.Debug("add route", "cidr", cidr, "dev", name)
	case "delete":
		// Ignore "no such process" / "not found" errors — the route may already be gone.
		_ = infra.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip route del %s dev %s 2>/dev/null || true", cidr, name))
		r.logger.Debug("delete route", "cidr", cidr, "dev", name)
	}
	return nil
}

func (r *routeProvisioner) ApplyIP(action, address, name string) error {
	switch action {
	case "add":
		// ip address replace 要求 CIDR 格式；若管理服务下发裸 IP（无前缀）则补 /32。
		if !strings.Contains(address, "/") {
			address = address + "/32"
		}
		if err := infra.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip address replace %s dev %s", address, name)); err != nil {
			return err
		}
		if err := infra.ExecCommand("/bin/sh", "-c", fmt.Sprintf("ip link set dev %s mtu %d up", name, infra.DefaultMTU)); err != nil {
			return err
		}
	}

	return nil
}

func (r *ruleProvisioner) Name() string {
	return "iptables"
}

func (r *ruleProvisioner) Provision(rule *infra.FirewallRule) error {
	inChain := "LATTICE-INGRESS"
	outChain := "LATTICE-EGRESS"

	// 1. 初始化链
	r.initChain(inChain, "INPUT", "-i")
	r.initChain(outChain, "OUTPUT", "-o")

	// 2. 清空旧规则 (Flush)
	if err := exec.Command("iptables", "-F", inChain).Run(); err != nil {
		return err
	}

	if err := exec.Command("iptables", "-F", outChain).Run(); err != nil {
		return err
	}

	// 3. 基础规则：允许 Established 流量（零信任回包保障）
	if err := exec.Command("iptables", "-A", inChain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT").Run(); err != nil {
		return err
	}

	if err := exec.Command("iptables", "-A", outChain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT").Run(); err != nil {
		return err
	}

	// 4. 应用 Ingress (源地址匹配 -s)
	for _, tr := range rule.Ingress {
		for _, ip := range tr.Peers {
			if err := r.addRule(inChain, "-s", ip, tr); err != nil {
				return err
			}
		}
	}

	// 5. 应用 Egress (目的地址匹配 -d)
	for _, tr := range rule.Egress {
		for _, ip := range tr.Peers {
			if err := r.addRule(outChain, "-d", ip, tr); err != nil {
				return err
			}
		}
	}

	// 6. 终极封口 (DROP)
	if err := exec.Command("iptables", "-A", inChain, "-j", "DROP").Run(); err != nil {
		return err
	}

	if err := exec.Command("iptables", "-A", outChain, "-j", "DROP").Run(); err != nil {
		return err
	}

	return nil
}

// 内部辅助：确保链存在并挂载
func (p *ruleProvisioner) initChain(chain, parent, flag string) {
	// 1. 创建链：使用 -w 避免锁竞争
	// 技巧：先检查链是否存在，或者直接运行并捕获错误
	cmd := exec.Command("iptables", "-w", "5", "-N", chain)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// 如果错误信息包含 "already exists"，说明链是好的，可以继续
		if strings.Contains(stderr.String(), "already exists") {
			p.logger.Debug("iptables chain already exists, skipping creation", "chain", chain)
		} else {
			p.logger.Error("init iptables failed", err, "stderr", stderr.String())
			// 如果不是因为已存在而失败，才 return
			return
		}
	}

	// 2. 检查是否已挂载到父链 (-C 是 Check)
	// 同样加上 -w 5
	checkCmd := exec.Command("iptables", "-w", "5", "-C", parent, flag, p.interfaceName, "-j", chain)
	if err := checkCmd.Run(); err != nil {
		// 如果 Check 失败（说明没挂载），则执行插入 (-I)
		insertCmd := exec.Command("iptables", "-w", "5", "-I", parent, "1", flag, p.interfaceName, "-j", chain)
		if err := insertCmd.Run(); err != nil {
			p.logger.Error("failed to bind chain to parent", err, "parent", parent)
		}
	}
}

// 内部辅助：添加单条规则。
// 当 Protocol 或 Port 未指定（零值）时，省略 -p/--dport，允许该 IP 的所有流量。
func (p *ruleProvisioner) addRule(chain, dir, ip string, tr infra.TrafficRule) error {
	var args []string
	if tr.Protocol != "" && tr.Port != 0 {
		args = []string{"-A", chain, dir, ip, "-p", strings.ToLower(tr.Protocol), "--dport", fmt.Sprintf("%d", tr.Port), "-j", "ACCEPT"}
	} else {
		args = []string{"-A", chain, dir, ip, "-j", "ACCEPT"}
	}
	return exec.Command("iptables", args...).Run()
}

func (p *ruleProvisioner) Cleanup() error {
	// 逻辑：删除挂载点 -> 清空链 -> 删除链
	return nil
}

// isRunningInContainer reports whether the process is running inside a container.
// It checks multiple indicators to cover Docker, OrbStack, Podman, containerd,
// and CRI-O runtimes:
//  1. /.dockerenv       — Docker
//  2. /run/.containerenv — Podman / OrbStack
//  3. /proc/1/cgroup    — kubepods / docker / containerd / crio entries
func isRunningInContainer() bool {
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		content := string(data)
		for _, kw := range []string{"docker", "kubepods", "containerd", "crio"} {
			if strings.Contains(content, kw) {
				return true
			}
		}
	}
	return false
}

// SetupNAT configures iptables NAT rules required when lattice runs inside a
// container acting as a VPN gateway. It is a no-op on bare-metal or VM
// deployments because ApplyRoute already installs the correct MASQUERADE rule
// on the default outbound interface.
// iptablesMu serializes SetupNAT iptables operations across concurrent callers.
var iptablesMu sync.Mutex

func (r *ruleProvisioner) SetupNAT(interfaceName string) error {
	if !isRunningInContainer() {
		return nil
	}

	// 每条规则先用 -C 检查是否已存在，避免重连时重复追加。
	type natRule struct {
		check string
		add   string
	}
	rules := []natRule{
		{
			check: fmt.Sprintf("iptables -w 5 -t nat -C POSTROUTING -o %s -j MASQUERADE", interfaceName),
			add:   fmt.Sprintf("iptables -w 5 -t nat -A POSTROUTING -o %s -j MASQUERADE", interfaceName),
		},
		{
			check: "iptables -w 5 -C FORWARD -j ACCEPT",
			add:   "iptables -w 5 -A FORWARD -j ACCEPT",
		},
		{
			check: fmt.Sprintf("iptables -w 5 -C FORWARD -i %s -o eth0 -m state --state RELATED,ESTABLISHED -j ACCEPT", interfaceName),
			add:   fmt.Sprintf("iptables -w 5 -A FORWARD -i %s -o eth0 -m state --state RELATED,ESTABLISHED -j ACCEPT", interfaceName),
		},
	}

	iptablesMu.Lock()
	defer iptablesMu.Unlock()
	for _, r := range rules {
		if err := infra.ExecCommand("/bin/sh", "-c", r.check); err != nil {
			// 规则不存在，添加
			if err := infra.ExecCommand("/bin/sh", "-c", r.add); err != nil {
				return err
			}
		}
	}

	log.Printf("Successfully configured iptables for %s", interfaceName)
	return nil
}
