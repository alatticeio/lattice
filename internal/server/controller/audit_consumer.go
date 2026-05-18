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

package controller

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	nats "github.com/nats-io/nats.go"
)

// FlowAuditMsg is the wire format published by natsAuditWriter.
type FlowAuditMsg struct {
	AgentID   string `json:"agentId"`
	TraceID   string `json:"traceId"`
	Direction string `json:"direction"`
	DstIP     string `json:"dstIp"`
	DstPort   int    `json:"dstPort"`
	Bytes     int64  `json:"bytes"`
	Ts        string `json:"ts"`
}

// AuditConsumer subscribes to NATS audit topics and persists flow events.
type AuditConsumer struct {
	nc         *nats.Conn
	flowEvents store.FlowEventRepository
	sub        *nats.Subscription
}

// NewAuditConsumer creates and starts the NATS flow audit consumer.
func NewAuditConsumer(nc *nats.Conn, flowEvents store.FlowEventRepository) (*AuditConsumer, error) {
	c := &AuditConsumer{nc: nc, flowEvents: flowEvents}
	sub, err := nc.Subscribe("lattice.audit.flow", c.handleFlowEvent)
	if err != nil {
		return nil, err
	}
	c.sub = sub
	log.Printf("[audit-consumer] subscribed to lattice.audit.flow")
	return c, nil
}

func (c *AuditConsumer) handleFlowEvent(msg *nats.Msg) {
	var m FlowAuditMsg
	if err := json.Unmarshal(msg.Data, &m); err != nil {
		log.Printf("[audit-consumer] unmarshal error: %v", err)
		return
	}
	ts, _ := time.Parse(time.RFC3339Nano, m.Ts)
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	e := &models.FlowEvent{
		TraceID:   m.TraceID,
		AgentID:   m.AgentID,
		Direction: m.Direction,
		DstIP:     m.DstIP,
		DstPort:   m.DstPort,
		Bytes:     m.Bytes,
		Ts:        ts,
	}
	if err := c.flowEvents.Write(context.Background(), e); err != nil {
		log.Printf("[audit-consumer] write flow event: %v", err)
	}
}

// Close unsubscribes from NATS.
func (c *AuditConsumer) Close() {
	if c.sub != nil {
		_ = c.sub.Unsubscribe()
	}
}
