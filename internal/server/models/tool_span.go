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

// ToolSpan records a single MCP tool call for observability.
type ToolSpan struct {
	Model
	TraceID    string    `gorm:"uniqueIndex;size:36" json:"traceId"`
	AgentID    string    `gorm:"index;size:128"     json:"agentId"`
	ParentID   string    `gorm:"index;size:128"     json:"parentId,omitempty"`
	Namespace  string    `gorm:"size:128"           json:"namespace"`
	Tool       string    `gorm:"size:128"           json:"tool"`
	Status     string    `gorm:"size:32"            json:"status"` // ok | error | blocked
	ErrorMsg   string    `gorm:"type:text"          json:"errorMsg,omitempty"`
	DurationMs int64     `json:"durationMs"`
	StartedAt  time.Time `gorm:"index"              json:"startedAt"`
}

func (ToolSpan) TableName() string { return "la_tool_spans" }
