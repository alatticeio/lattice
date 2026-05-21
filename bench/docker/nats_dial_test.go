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
	"sync"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"
)

// BenchmarkNATSDial measures NATS publish→subscribe round-trip latency using
// an embedded in-process server — no external NATS process required.
// This represents the signaling overhead for one peer-update message.
func BenchmarkNATSDial(b *testing.B) {
	// Start embedded NATS server on a random port.
	opts := &natsserver.Options{
		Host:           "127.0.0.1",
		Port:           natsserver.RANDOM_PORT,
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 4096,
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

	payload := []byte(`{"eventType":2,"configVersion":"v1","peer":{"name":"bench-peer"}}`)
	subject := "lattice.bench.signal"

	var wg sync.WaitGroup
	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		wg.Done()
	})
	if err != nil {
		b.Fatal(err)
	}
	_ = nc.Flush()

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
