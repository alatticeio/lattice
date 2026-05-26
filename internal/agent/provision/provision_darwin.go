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
	// Ensure pf is enabled
	exec.Command("sudo", "pfctl", "-e").Run() //nolint:errcheck

	// Check if anchor is already in the main ruleset
	out, _ := exec.Command("sudo", "pfctl", "-sr").Output()
	if strings.Contains(string(out), `anchor "`+anchor+`"`) {
		return nil
	}

	// Append anchor to the main ruleset and reload
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

	// 1. Default deny (zero-trust catch-all)
	// Note: PF is last-match-wins, block rules must be written before pass rules as a fallback
	iface := r.interfaceName
	fmt.Fprintf(&sb, "block in on %s all\n", iface)
	fmt.Fprintf(&sb, "block out on %s all\n", iface)

	// 2. Generate PF rule string
	// When Protocol or Port is not specified (zero value), omit proto/port to allow all traffic from that IP.
	// Ingress: pass in [proto tcp] from {IP1} [to any port 80]
	for _, tr := range rule.Ingress {
		if len(tr.Peers) == 0 {
			// Empty peer list means allow-all for this direction
			fmt.Fprintf(&sb, "pass in on %s all\n", iface)
			continue
		}
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
		if len(tr.Peers) == 0 {
			// Empty peer list means allow-all for this direction
			fmt.Fprintf(&sb, "pass out on %s all\n", iface)
			continue
		}
		ips := "{" + strings.Join(tr.Peers, ", ") + "}"
		if tr.Protocol != "" && tr.Port != 0 {
			fmt.Fprintf(&sb, "pass out proto %s from any to %s port %d\n",
				strings.ToLower(tr.Protocol), ips, tr.Port)
		} else {
			fmt.Fprintf(&sb, "pass out to %s\n", ips)
		}
	}

	// 3. Write rules to a temp file and load into anchor
	tmpFile := "/tmp/lattice.pf"
	if err := os.WriteFile(tmpFile, []byte(sb.String()), 0644); err != nil {
		return err
	}

	// Load into the specific anchor using pfctl, without affecting other system rules
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
