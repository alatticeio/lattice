//go:build pro

// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runsc_test

import (
	"encoding/json"
	"testing"

	"github.com/alatticeio/lattice/internal/agent/runsc"
)

func TestOCISpec(t *testing.T) {
	mgr := &runsc.Manager{}
	mgr.SetConfig(runsc.Config{
		SandboxID:   "test-sandbox",
		RootFS:      "/rootfs",
		AgentBinary: "/usr/bin/myagent",
		AgentArgs:   []string{"--flag", "val"},
		ServerURL:   "http://ctrl:8080",
		Token:       "tok-abc",
	})

	spec := mgr.OCISpec()

	// network namespace must NOT be present (container shares pod netns)
	linux, ok := spec["linux"].(map[string]any)
	if !ok {
		t.Fatal("missing linux section")
	}
	namespaces, ok := linux["namespaces"].([]map[string]string)
	if !ok {
		t.Fatal("missing linux.namespaces")
	}
	for _, ns := range namespaces {
		if ns["type"] == "network" {
			t.Error("network namespace must not appear in OCI spec (shared pod netns)")
		}
	}

	// capabilities must include CAP_NET_ADMIN
	caps, ok := linux["capabilities"].(map[string][]string)
	if !ok {
		t.Fatal("missing linux.capabilities")
	}
	hasNetAdmin := false
	for _, c := range caps["effective"] {
		if c == "CAP_NET_ADMIN" {
			hasNetAdmin = true
		}
	}
	if !hasNetAdmin {
		t.Error("expected CAP_NET_ADMIN in effective capabilities")
	}

	// TUN device must be present so wireguard-go can create wg0
	devs, ok := linux["devices"].([]map[string]any)
	if !ok {
		t.Fatal("missing linux.devices")
	}
	hasTUN := false
	for _, d := range devs {
		if d["path"] == "/dev/net/tun" {
			hasTUN = true
			break
		}
	}
	if !hasTUN {
		t.Error("expected /dev/net/tun in OCI spec linux.devices")
	}

	// process.args must start with "lattice sandbox agent"
	proc, ok := spec["process"].(map[string]any)
	if !ok {
		t.Fatal("missing process section")
	}
	args, ok := proc["args"].([]string)
	if !ok {
		t.Fatal("process.args is not []string")
	}
	if len(args) < 3 || args[0] != "lattice" || args[1] != "sandbox" || args[2] != "agent" {
		t.Errorf("expected process.args to start with [lattice sandbox agent], got %v", args)
	}
	// --name, --server-url, --token must appear
	argsJSON, _ := json.Marshal(args)
	argsStr := string(argsJSON)
	for _, needle := range []string{"--name", "--server-url", "--token", "--", "/usr/bin/myagent", "--flag"} {
		found := false
		for _, a := range args {
			if a == needle {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in process.args, got %s", needle, argsStr)
		}
	}

	// /etc/resolv.conf must be bind-mounted from the host pod so gVisor
	// DNS resolution uses the correct CoreDNS nameserver.
	mounts, ok := spec["mounts"].([]map[string]any)
	if !ok {
		t.Fatal("missing mounts section")
	}
	hasResolvConf := false
	hasLatticeCfg := false
	for _, m := range mounts {
		switch m["destination"] {
		case "/etc/resolv.conf":
			hasResolvConf = true
		case "/etc/lattice":
			hasLatticeCfg = true
		}
	}
	if !hasResolvConf {
		t.Error("expected /etc/resolv.conf bind mount in OCI spec")
	}
	if !hasLatticeCfg {
		t.Error("expected /etc/lattice bind mount in OCI spec (for sandbox credentials)")
	}
}
