//go:build pro

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

package cmd

import (
	"log"

	"github.com/alatticeio/lattice-shim/shim"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
)

// sandboxCloser wraps the gVisor Sandbox for the CLI's deferred cleanup.
type sandboxCloser struct {
	sb *gvisor.Sandbox
}

func (c *sandboxCloser) Close() {
	if c.sb != nil {
		c.sb.Close()
	}
}

// createSandbox creates a gVisor sandbox with the given parameters.
// agentJWT may be empty (sandbox runs without API auth).
func createSandbox(name, localIP, agentJWT string) (*sandboxCloser, error) {
	var policyChecker shim.PolicyChecker
	var auditWriter shim.AuditWriter

	if agentJWT != "" {
		// Wrap the log-based audit writer through the AuditAdapter.
		// In production, PolicyAdapter would be backed by the Lattice
		// policy engine.
		log.Printf("[sandbox] agent %q: using allow-all policy (policy engine not wired)", name)
		auditWriter = gvisor.NewAuditAdapter(name, newLogAuditAdapter(name))
	}

	cfg := gvisor.Config{
		ID:            name,
		LocalIP:       localIP,
		PolicyChecker: policyChecker,
		AuditWriter:   auditWriter,
		// WireGuardBind is nil — the sandbox works locally without WG.
		// Enable with --wg flag and the existing infra.DefaultBind.
	}

	sb, err := gvisor.New(cfg)
	if err != nil {
		return nil, err
	}
	return &sandboxCloser{sb: sb}, nil
}

// logAuditAdapter writes audit events to stderr via the log package.
type logAuditAdapter struct {
	id string
}

func newLogAuditAdapter(id string) *logAuditAdapter {
	return &logAuditAdapter{id: id}
}

func (a *logAuditAdapter) WriteAudit(agentID string, event shim.AuditEvent) error {
	log.Printf("[audit] sandbox=%s identity=%s dst=%s:%d protocol=%s verdict=%s",
		a.id, event.Identity, event.DstIP, event.DstPort, event.Protocol, event.Verdict)
	return nil
}

// compile-time checks.
var _ gvisor.AuditEventWriter = (*logAuditAdapter)(nil)
