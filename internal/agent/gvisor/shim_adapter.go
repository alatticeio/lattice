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

package gvisor

import (
	"net"

	"github.com/alatticeio/lattice-shim/shim"
)

// PolicyAdapter implements shim.PolicyChecker by delegating to the Lattice
// policy engine. It is injected into the gVisor netstack so that every
// socket-level dial is checked against the agent's allowed destinations.
type PolicyAdapter struct {
	sandboxID string
	checkers  []PolicyRuleChecker
}

// PolicyRuleChecker is implemented by callers that have access to the
// Lattice policy engine (CRD or local cache).
type PolicyRuleChecker interface {
	Allow(agentID string, dstIP net.IP, dstPort uint16) bool
}

// NewPolicyAdapter creates a PolicyAdapter that queries the provided
// checkers in order. The first Allow=true result short-circuits.
func NewPolicyAdapter(sandboxID string, checkers ...PolicyRuleChecker) *PolicyAdapter {
	return &PolicyAdapter{sandboxID: sandboxID, checkers: checkers}
}

// Allow implements shim.PolicyChecker.
func (a *PolicyAdapter) Allow(identity string, dstIP net.IP, dstPort uint16) bool {
	for _, c := range a.checkers {
		if c.Allow(a.sandboxID, dstIP, dstPort) {
			return true
		}
	}
	return false
}

// Compile-time check.
var _ shim.PolicyChecker = (*PolicyAdapter)(nil)

// AuditAdapter implements shim.AuditWriter by sending events to the
// Lattice audit pipeline (batch + POST to control plane).
type AuditAdapter struct {
	sandboxID string
	writers   []AuditEventWriter
}

// AuditEventWriter receives a single audit event.
type AuditEventWriter interface {
	WriteAudit(agentID string, event shim.AuditEvent) error
}

// NewAuditAdapter creates an AuditAdapter that fans out events to all
// provided writers (e.g. local ring buffer + remote API).
func NewAuditAdapter(sandboxID string, writers ...AuditEventWriter) *AuditAdapter {
	return &AuditAdapter{sandboxID: sandboxID, writers: writers}
}

// Write implements shim.AuditWriter.
func (a *AuditAdapter) Write(event shim.AuditEvent) error {
	for _, w := range a.writers {
		if err := w.WriteAudit(a.sandboxID, event); err != nil {
			return err
		}
	}
	return nil
}

// Compile-time check.
var _ shim.AuditWriter = (*AuditAdapter)(nil)
