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
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/agent/infra"
	natsserver "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"
)

// BenchmarkSandboxBootstrap measures the NATS signal round-trip for a full
// JoinNetwork bootstrap message. This is a proxy for sandbox registration
// latency without requiring a running latticed process.
func BenchmarkSandboxBootstrap(b *testing.B) {
	opts := &natsserver.Options{
		Host:   "127.0.0.1",
		Port:   natsserver.RANDOM_PORT,
		NoLog:  true,
		NoSigs: true,
	}
	srv, err := natsserver.NewServer(opts)
	if err != nil {
		b.Fatal(err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		b.Fatal("NATS server did not become ready")
	}
	defer srv.Shutdown()

	url := srv.ClientURL()
	nc, err := nats.Connect(url)
	if err != nil {
		b.Fatal(err)
	}
	defer nc.Close()

	addr := "100.64.0.1"
	msg := &infra.Message{
		EventType:     infra.EventTypeJoinNetwork,
		ConfigVersion: "v1",
		Current: &infra.Peer{
			Name:      "bench-sandbox",
			PublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
			Address:   &addr,
		},
		Network: &infra.Network{
			NetworkId:   "net-bench",
			NetworkName: "bench",
			Address:     "100.64.0.0/10",
			Port:        51820,
		},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		b.Fatal(err)
	}

	subject := "lattice.bench.bootstrap"
	var wg sync.WaitGroup
	_, err = nc.Subscribe(subject, func(m *nats.Msg) {
		wg.Done()
	})
	if err != nil {
		b.Fatal(err)
	}
	_ = nc.Flush()

	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		if err := nc.Publish(subject, payload); err != nil {
			b.Fatal(err)
		}
		wg.Wait()
	}
}
