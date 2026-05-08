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
		// ip address replace requires CIDR format; append /32 if the management service sends a bare IP (no prefix).
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

	r.logger.Info("provisioning iptables rules",
		"ingressRules", len(rule.Ingress),
		"egressRules", len(rule.Egress),
	)

	// 1. Initialize chains
	r.initChain(inChain, "INPUT", "-i")
	r.initChain(outChain, "OUTPUT", "-o")

	// 2. Flush old rules
	r.logger.Debug("flushing iptables chains", "ingress", inChain, "egress", outChain)
	if err := exec.Command("iptables", "-F", inChain).Run(); err != nil {
		r.logger.Error("failed to flush ingress chain", err, "chain", inChain)
		return err
	}

	if err := exec.Command("iptables", "-F", outChain).Run(); err != nil {
		r.logger.Error("failed to flush egress chain", err, "chain", outChain)
		return err
	}

	// 3. Base rule: allow Established traffic (zero-trust return traffic guarantee)
	if err := exec.Command("iptables", "-A", inChain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT").Run(); err != nil {
		return err
	}

	if err := exec.Command("iptables", "-A", outChain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT").Run(); err != nil {
		return err
	}

	// 4. Apply Ingress rules (source address match -s)
	for _, tr := range rule.Ingress {
		for _, ip := range tr.Peers {
			r.logger.Debug("adding ingress rule", "chain", inChain, "src", ip, "protocol", tr.Protocol, "port", tr.Port, "action", tr.Action)
			if err := r.addRule(inChain, "-s", ip, tr); err != nil {
				r.logger.Error("failed to add ingress rule", err, "chain", inChain, "src", ip)
				return err
			}
		}
	}

	// 5. Apply Egress rules (destination address match -d)
	for _, tr := range rule.Egress {
		for _, ip := range tr.Peers {
			r.logger.Debug("adding egress rule", "chain", outChain, "dst", ip, "protocol", tr.Protocol, "port", tr.Port, "action", tr.Action)
			if err := r.addRule(outChain, "-d", ip, tr); err != nil {
				r.logger.Error("failed to add egress rule", err, "chain", outChain, "dst", ip)
				return err
			}
		}
	}

	// 6. Ultimate default deny (DROP)
	if err := exec.Command("iptables", "-A", inChain, "-j", "DROP").Run(); err != nil {
		return err
	}

	if err := exec.Command("iptables", "-A", outChain, "-j", "DROP").Run(); err != nil {
		return err
	}

	r.logger.Info("iptables rules applied", "ingress", inChain, "egress", outChain)
	return nil
}

// Internal helper: ensure chain exists and is attached
func (p *ruleProvisioner) initChain(chain, parent, flag string) {
	// 1. Create chain: use -w to avoid lock contention
	// Tip: either check if chain exists first, or run directly and catch the error
	cmd := exec.Command("iptables", "-w", "5", "-N", chain)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// If the error contains "already exists", the chain is fine, we can proceed
		if strings.Contains(stderr.String(), "already exists") {
			p.logger.Debug("iptables chain already exists, skipping creation", "chain", chain)
		} else {
			p.logger.Error("init iptables failed", err, "stderr", stderr.String())
			// Only return if the failure is not due to "already exists"
			return
		}
	}

	// 2. Check if already attached to parent chain (-C is Check)
	// Also add -w 5
	checkCmd := exec.Command("iptables", "-w", "5", "-C", parent, flag, p.interfaceName, "-j", chain)
	if err := checkCmd.Run(); err != nil {
		// If Check fails (not attached), perform insert (-I)
		insertCmd := exec.Command("iptables", "-w", "5", "-I", parent, "1", flag, p.interfaceName, "-j", chain)
		if err := insertCmd.Run(); err != nil {
			p.logger.Error("failed to bind chain to parent", err, "parent", parent)
		}
	}
}

// Internal helper: add a single rule.
// When Protocol or Port is not specified (zero value), omit -p/--dport to allow all traffic from that IP.
func (p *ruleProvisioner) addRule(chain, dir, ip string, tr infra.TrafficRule) error {
	target := tr.Action
	if target == "" {
		target = "ACCEPT"
	}
	var args []string
	if tr.Protocol != "" && tr.Port != 0 {
		args = []string{"-A", chain, dir, ip, "-p", strings.ToLower(tr.Protocol), "--dport", fmt.Sprintf("%d", tr.Port), "-j", target}
	} else {
		args = []string{"-A", chain, dir, ip, "-j", target}
	}
	p.logger.Debug("iptables", "cmd", fmt.Sprintf("iptables %s", strings.Join(args, " ")))
	if err := exec.Command("iptables", args...).Run(); err != nil {
		p.logger.Error("iptables rule failed", err, "args", args)
		return err
	}
	return nil
}

func (p *ruleProvisioner) Cleanup() error {
	// Logic: remove attachment points -> flush chains -> delete chains
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

	// Check each rule with -C first to avoid duplicate appends on reconnection.
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
			// Rule does not exist, add it
			if err := infra.ExecCommand("/bin/sh", "-c", r.add); err != nil {
				return err
			}
		}
	}

	log.Printf("Successfully configured iptables for %s", interfaceName)
	return nil
}
