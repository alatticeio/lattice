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

package models

import "time"

// FlowEvent records a single network flow observed by the gVisor AuditWriter.
// Linked to ToolSpan via TraceID for tool call → network traffic correlation.
type FlowEvent struct {
	Model
	TraceID   string    `gorm:"index;size:36"  json:"traceId"`
	AgentID   string    `gorm:"index;size:128" json:"agentId"`
	Direction string    `gorm:"size:16"        json:"direction"` // egress | ingress
	DstIP     string    `gorm:"size:64"        json:"dstIp"`
	DstPort   int       `json:"dstPort"`
	Bytes     int64     `json:"bytes"`
	Ts        time.Time `gorm:"index"          json:"ts"`
}

func (FlowEvent) TableName() string { return "la_flow_events" }
