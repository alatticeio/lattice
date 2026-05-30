//go:build linux

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

package sandbox

import (
	"fmt"
	"os/exec"
)

const (
	sandboxNetnsName = "lt-sandbox"       // named netns in /var/run/netns/
	hostVethName     = "lts-host"         // host-side veth interface
	agentVethName    = "lts-agent"        // agent-side veth interface (lives in netns)
	hostVethAddr     = "169.254.100.1/30" // host-side link-local IP
	agentVethAddr    = "169.254.100.2/30" // agent-side link-local IP
	hostVethIP       = "169.254.100.1"    // host veth IP without prefix (gateway for agent)
	proxyPort        = "18080"            // MCP proxy listen port in host netns
)

// setupSandboxNetns creates an isolated network namespace for the sandbox agent
// and wires it to the host network via a veth pair.
//
// After this call:
//   - /var/run/netns/lt-sandbox exists (named netns)
//   - lts-host (169.254.100.1/30) in host netns, UP
//   - lts-agent (169.254.100.2/30) in lt-sandbox netns, UP
//   - default route in lt-sandbox → 169.254.100.1 (host veth)
//   - IP forwarding enabled on host
//
// Returns the proxy listen address the MCP proxy should bind to.
// Call teardownSandboxNetns() to clean up.
func setupSandboxNetns() (proxyAddr string, err error) {
	// Clean up any leftover state from a previous crashed run.
	_ = teardownSandboxNetns()

	steps := [][]string{
		// 1. Create named netns (bind-mounts at /var/run/netns/lt-sandbox)
		{"ip", "netns", "add", sandboxNetnsName},
		// 2. Create veth pair in host netns
		{"ip", "link", "add", hostVethName, "type", "veth", "peer", "name", agentVethName},
		// 3. Move agent end into the netns
		{"ip", "link", "set", agentVethName, "netns", sandboxNetnsName},
		// 4. Configure host side
		{"ip", "addr", "add", hostVethAddr, "dev", hostVethName},
		{"ip", "link", "set", hostVethName, "up"},
		// 5. Configure agent side (inside the netns)
		{"ip", "netns", "exec", sandboxNetnsName, "ip", "link", "set", "lo", "up"},
		{"ip", "netns", "exec", sandboxNetnsName, "ip", "addr", "add", agentVethAddr, "dev", agentVethName},
		{"ip", "netns", "exec", sandboxNetnsName, "ip", "link", "set", agentVethName, "up"},
		// 6. Default route in agent netns: everything goes to host veth
		{"ip", "netns", "exec", sandboxNetnsName, "ip", "route", "add", "default", "via", hostVethIP},
		// 7. Enable IP forwarding so host can route agent traffic to wf0/internet
		{"sysctl", "-w", "net.ipv4.ip_forward=1"},
	}

	for _, args := range steps {
		if out, runErr := exec.Command(args[0], args[1:]...).CombinedOutput(); runErr != nil {
			_ = teardownSandboxNetns()
			return "", fmt.Errorf("sandbox netns setup (%v): %w\n%s", args, runErr, out)
		}
	}

	return hostVethIP + ":" + proxyPort, nil
}

// teardownSandboxNetns removes the named netns and veth interfaces.
// Idempotent — safe to call even if setup was partial or already torn down.
func teardownSandboxNetns() error {
	// Delete host veth (automatically deletes the peer inside the netns).
	// Ignore errors — interface may not exist.
	_ = exec.Command("ip", "link", "del", hostVethName).Run()
	// Delete named netns.
	_ = exec.Command("ip", "netns", "del", sandboxNetnsName).Run()
	return nil
}

// sandboxNetnsPath returns the filesystem path of the named netns.
// Used by forkAgent to open the netns fd for Setns().
func sandboxNetnsPath() string {
	return "/var/run/netns/" + sandboxNetnsName
}
