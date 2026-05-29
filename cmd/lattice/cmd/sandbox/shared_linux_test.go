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
	"context"
	"net"
	"testing"
	"time"

	shim "github.com/alatticeio/lattice-shim/shim"
)

func stubListener(t *testing.T) (addr string, accepted <-chan struct{}) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	ch := make(chan struct{}, 1)
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
		ch <- struct{}{}
	}()
	return ln.Addr().String(), ch
}

func TestPolicyDialer_NilChecker_Allows(t *testing.T) {
	addr, accepted := stubListener(t)
	d := &policyDialer{identity: "agent-1", checker: nil, auditor: nil}
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("expected allow, got err: %v", err)
	}
	conn.Close()
	<-accepted
}

func TestPolicyDialer_PolicyAllow_Connects(t *testing.T) {
	addr, accepted := stubListener(t)
	policy := shim.EgressPolicy{
		DefaultDeny:  true,
		AllowedCIDRs: []net.IPNet{mustParseCIDR("127.0.0.0/8")},
	}
	d := &policyDialer{
		identity: "agent-1",
		checker:  shim.NewEgressFilter(policy),
		auditor:  nil,
	}
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("expected allow for 127.x.x.x, got err: %v", err)
	}
	conn.Close()
	<-accepted
}

func TestPolicyDialer_PolicyDeny_Blocks(t *testing.T) {
	addr, _ := stubListener(t)
	policy := shim.EgressPolicy{
		DefaultDeny:  true,
		AllowedCIDRs: []net.IPNet{mustParseCIDR("10.0.0.0/8")},
	}
	d := &policyDialer{
		identity: "agent-1",
		checker:  shim.NewEgressFilter(policy),
		auditor:  nil,
	}
	_, err := d.DialContext(context.Background(), "tcp", addr)
	if err == nil {
		t.Fatal("expected deny for 127.x.x.x when only 10.x allowed")
	}
}

func TestPolicyDialer_Audit_WritesEvent(t *testing.T) {
	addr, accepted := stubListener(t)

	var written []shim.AuditEvent
	auditor := &testAuditor{onWrite: func(e shim.AuditEvent) { written = append(written, e) }}

	d := &policyDialer{identity: "agent-1", checker: nil, auditor: auditor}
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	conn.Close()
	<-accepted

	if len(written) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(written))
	}
	if written[0].Verdict != shim.VerdictAllow {
		t.Errorf("expected allow verdict, got %s", written[0].Verdict)
	}
	if written[0].Identity != "agent-1" {
		t.Errorf("expected identity agent-1, got %s", written[0].Identity)
	}
}

func TestPolicyDialer_Audit_WritesDenyEvent(t *testing.T) {
	addr, _ := stubListener(t)

	var written []shim.AuditEvent
	auditor := &testAuditor{onWrite: func(e shim.AuditEvent) { written = append(written, e) }}
	policy := shim.EgressPolicy{DefaultDeny: true}

	d := &policyDialer{
		identity: "agent-1",
		checker:  shim.NewEgressFilter(policy),
		auditor:  auditor,
	}
	_, _ = d.DialContext(context.Background(), "tcp", addr)

	if len(written) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(written))
	}
	if written[0].Verdict != shim.VerdictDrop {
		t.Errorf("expected drop verdict, got %s", written[0].Verdict)
	}
}

func TestPolicyDialer_Hostname_DeniedWhenCheckerSet(t *testing.T) {
	policy := shim.EgressPolicy{
		DefaultDeny:  true,
		AllowedCIDRs: []net.IPNet{mustParseCIDR("127.0.0.0/8")},
	}
	d := &policyDialer{
		identity: "agent-1",
		checker:  shim.NewEgressFilter(policy),
		auditor:  nil,
	}
	// hostname (not IP) should be denied when checker is set
	_, err := d.DialContext(context.Background(), "tcp", "localhost:80")
	if err == nil {
		t.Fatal("expected hostname to be denied when checker is set, got nil error")
	}
}

func TestPolicyDialer_InvalidAddr_ReturnsError(t *testing.T) {
	d := &policyDialer{identity: "agent-1", checker: nil, auditor: nil}
	_, err := d.DialContext(context.Background(), "tcp", "not-valid-addr-no-port")
	if err == nil {
		t.Fatal("expected error for malformed addr, got nil")
	}
}

func TestForkWithProxy_SetsAllProxy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := forkWithProxy(ctx, cancel, []string{"sh", "-c", "echo $ALL_PROXY"}, "socks5h://127.0.0.1:9999")
	if err != nil {
		t.Fatalf("forkWithProxy returned err: %v", err)
	}
}

func TestForkWithProxy_ChildExitCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := forkWithProxy(ctx, cancel, []string{"true"}, "socks5h://127.0.0.1:9999")
	if err != nil {
		t.Fatalf("expected nil error for exit 0, got: %v", err)
	}
}

type testAuditor struct {
	onWrite func(shim.AuditEvent)
}

func (a *testAuditor) Write(e shim.AuditEvent) error {
	a.onWrite(e)
	return nil
}

func mustParseCIDR(s string) net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return *n
}
