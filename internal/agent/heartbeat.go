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

package agent

import (
	"context"
	"encoding/json"
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
	"time"
)

const heartbeatInterval = 30 * time.Second
const heartbeatTimeout = 5 * time.Second

type heartbeatPayload struct {
	AppID string `json:"appId"`
}

// StartHeartbeat sends a periodic heartbeat to the management server via NATS
// so the server can track the node's online status.
// It runs until ctx is cancelled and is safe to run in a goroutine.
func (c *Node) StartHeartbeat(ctx context.Context) {
	logger := log.GetLogger("heartbeat")
	appId := config.Conf.AppId

	data, err := json.Marshal(heartbeatPayload{AppID: appId})
	if err != nil {
		logger.Error("marshal heartbeat payload failed", err)
		return
	}

	send := func() {
		hbCtx, cancel := context.WithTimeout(ctx, heartbeatTimeout)
		defer cancel()
		if _, err := c.ctrClient.RequestNats(hbCtx, "lattice.signals.peer", "heartbeat", data); err != nil {
			logger.Warn("heartbeat send failed", "err", err)
		}
	}

	// send immediately on startup so the node appears online right away
	send()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send()
		}
	}
}
