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

package server

import (
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/controller"
	nats "github.com/nats-io/nats.go"
)

// initFlowAuditConsumer connects to NATS and starts the flow audit consumer (PRO only).
// Returns nil if natsURL is empty, the store is nil, or connection fails.
func initFlowAuditConsumer(natsURL string, st store.Store) interface{ Close() } {
	logger := log.GetLogger("audit-consumer")
	if natsURL == "" || st == nil {
		return nil
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		logger.Warn("audit consumer NATS connect failed", "url", natsURL, "err", err)
		return nil
	}
	consumer, err := controller.NewAuditConsumer(nc, st.FlowEvents())
	if err != nil {
		logger.Warn("audit consumer init failed", "err", err)
		_ = nc.Drain()
		return nil
	}
	logger.Info("flow audit consumer started", "nats", natsURL)
	return consumer
}
