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
	})

	spec := mgr.OCISpec()

	// network namespace must NOT be present (container shares pod netns).
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

	// capabilities must NOT include CAP_NET_ADMIN (Phase 1 networking runs
	// on the pod kernel, not inside gVisor).
	if caps, ok := linux["capabilities"].(map[string][]string); ok {
		for _, c := range caps["effective"] {
			if c == "CAP_NET_ADMIN" {
				t.Error("CAP_NET_ADMIN must not appear in OCI spec (networking handled by pod kernel)")
			}
		}
	}

	// TUN device must NOT be present (real TUN is on the pod kernel).
	if devs, ok := linux["devices"].([]map[string]any); ok {
		for _, d := range devs {
			if d["path"] == "/dev/net/tun" {
				t.Error("/dev/net/tun must not appear in OCI spec (real TUN is on pod kernel)")
			}
		}
	}

	// process.args must start with the AI agent binary (no longer "lattice sandbox agent").
	proc, ok := spec["process"].(map[string]any)
	if !ok {
		t.Fatal("missing process section")
	}
	args, ok := proc["args"].([]string)
	if !ok || len(args) < 1 {
		t.Fatal("process.args is empty or not []string")
	}
	if args[0] != "/usr/bin/myagent" {
		t.Errorf("expected process.args[0]=/usr/bin/myagent, got %v", args[0])
	}
	// "--flag" and "val" must appear as agent args.
	for _, needle := range []string{"--flag", "val"} {
		found := false
		for _, a := range args {
			if a == needle {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in process.args, got %v", needle, args)
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
	if hasLatticeCfg {
		t.Error("/etc/lattice bind mount must not appear (credentials live on pod kernel)")
	}
}
