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

package benchdocker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pion/ice/v4"
)

// BenchmarkICEDialLocal measures ICE handshake time between two in-process
// agents over loopback (no NAT). Represents the best-case TTFH for LAN peers.
// Target: < 3 s (spec SLA for LAN).
func BenchmarkICEDialLocal(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		runICEHandshake(b)
	}
}

// copyCandidate returns a new Candidate by round-tripping through the SDP
// string representation. This avoids sharing the underlying candidateBase
// struct (and its closeCh) between agents, which would cause a
// "close of closed channel" panic in pion/ice v4 when both agents close.
func copyCandidate(c ice.Candidate) (ice.Candidate, error) {
	return ice.UnmarshalCandidate(c.Marshal())
}

// runICEHandshake performs one full ICE gather + dial/accept cycle and reports
// the elapsed time. It is kept in its own function so that local variables and
// closure captures are properly scoped across multiple benchmark iterations.
func runICEHandshake(b *testing.B) {
	b.Helper()

	start := time.Now()

	// Fresh agents each call; sharing state across calls causes
	// "close of closed channel" panics in pion/ice v4 agent teardown.
	opts := []ice.AgentOption{
		ice.WithNetworkTypes([]ice.NetworkType{ice.NetworkTypeUDP4}),
		ice.WithCandidateTypes([]ice.CandidateType{ice.CandidateTypeHost}),
	}

	aAgent, err := ice.NewAgentWithOptions(opts...)
	if err != nil {
		b.Fatal(err)
	}
	bAgent, err := ice.NewAgentWithOptions(opts...)
	if err != nil {
		b.Fatal(err)
	}

	aUfrag, aPwd, err := aAgent.GetLocalUserCredentials()
	if err != nil {
		b.Fatal(err)
	}
	bUfrag, bPwd, err := bAgent.GetLocalUserCredentials()
	if err != nil {
		b.Fatal(err)
	}

	// aDone / bDone are closed when each agent's gathering completes (nil sentinel).
	aDone := make(chan struct{})
	bDone := make(chan struct{})
	var aOnce, bOnce sync.Once

	// OnCandidate callbacks: deep-copy each candidate before passing to the
	// peer agent. Sharing the same candidateBase pointer between two agents
	// causes a double-close panic in pion/ice v4 when both agents shut down.
	if err := aAgent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			aOnce.Do(func() { close(aDone) })
			return
		}
		cp, cpErr := copyCandidate(c)
		if cpErr != nil {
			b.Logf("copyCandidate A: %v", cpErr)
			return
		}
		if addErr := bAgent.AddRemoteCandidate(cp); addErr != nil {
			b.Logf("A→B AddRemoteCandidate: %v", addErr)
		}
	}); err != nil {
		b.Fatal(err)
	}
	if err := bAgent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			bOnce.Do(func() { close(bDone) })
			return
		}
		cp, cpErr := copyCandidate(c)
		if cpErr != nil {
			b.Logf("copyCandidate B: %v", cpErr)
			return
		}
		if addErr := aAgent.AddRemoteCandidate(cp); addErr != nil {
			b.Logf("B→A AddRemoteCandidate: %v", addErr)
		}
	}); err != nil {
		b.Fatal(err)
	}

	if err := aAgent.GatherCandidates(); err != nil {
		b.Fatal(err)
	}
	if err := bAgent.GatherCandidates(); err != nil {
		b.Fatal(err)
	}

	// Wait for both agents to finish gathering before dialing.
	gatherCtx, gatherCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer gatherCancel()
	select {
	case <-aDone:
	case <-gatherCtx.Done():
		b.Fatal("aAgent gather timed out")
	}
	select {
	case <-bDone:
	case <-gatherCtx.Done():
		b.Fatal("bAgent gather timed out")
	}

	// Drain each agent's task loop so that all queued AddRemoteCandidate tasks
	// have been processed before Dial/Accept begin connectivity checks.
	// GetRemoteCandidates submits a synchronous task to the same loop, so
	// returning from it guarantees all previously enqueued tasks are done.
	if _, err := aAgent.GetRemoteCandidates(); err != nil {
		b.Logf("aAgent.GetRemoteCandidates: %v", err)
	}
	if _, err := bAgent.GetRemoteCandidates(); err != nil {
		b.Logf("bAgent.GetRemoteCandidates: %v", err)
	}

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dialCancel()

	connCh := make(chan error, 2)
	go func() {
		conn, dialErr := aAgent.Dial(dialCtx, bUfrag, bPwd)
		if dialErr == nil {
			conn.Close() //nolint:errcheck
		}
		connCh <- dialErr
	}()
	go func() {
		conn, acceptErr := bAgent.Accept(dialCtx, aUfrag, aPwd)
		if acceptErr == nil {
			conn.Close() //nolint:errcheck
		}
		connCh <- acceptErr
	}()

	for j := 0; j < 2; j++ {
		if connErr := <-connCh; connErr != nil {
			b.Fatalf("ICE connect failed: %v", connErr)
		}
	}

	elapsed := time.Since(start)
	b.ReportMetric(float64(elapsed.Milliseconds()), "handshake_ms/op")

	_ = aAgent.Close()
	_ = bAgent.Close()
}
