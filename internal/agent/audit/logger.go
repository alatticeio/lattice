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

package audit

import (
	"time"
)

// AuditEntry 表示一条审计日志记录
type AuditEntry struct {
	Timestamp    time.Time      `json:"timestamp"`
	AgentID      string         `json:"agent_id"`
	PeerID       string         `json:"peer_id"`
	PeerIdentity string         `json:"peer_identity"`
	Action       string         `json:"action"`
	Tool         string         `json:"tool"`
	Params       map[string]any `json:"params,omitempty"`
	Result       string         `json:"result"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Duration     int64          `json:"duration_ms,omitempty"`
	Sandbox      string         `json:"sandbox,omitempty"`
	PolicyMatch  string         `json:"policy_matched,omitempty"`
	RiskScore    int            `json:"risk_score,omitempty"`
	CheckResults CheckResult    `json:"checks"`
}

// CheckResult 记录安全检查结果
type CheckResult struct {
	IdentityValid bool `json:"identity_valid"`
	ToolAllowed   bool `json:"tool_allowed"`
	ParamsSafe    bool `json:"params_safe"`
}

// Logger 定义审计日志接口
type Logger interface {
	// Log 记录一条审计日志
	Log(entry AuditEntry)

	// LogError 记录一条错误审计日志
	LogError(entry AuditEntry, err error)

	// Close 关闭日志器
	Close() error
}

// noopLogger 是一个空操作的日志器，用于测试
type noopLogger struct{}

func NewNoopLogger() Logger {
	return &noopLogger{}
}

func (l *noopLogger) Log(entry AuditEntry)                 {}
func (l *noopLogger) LogError(entry AuditEntry, err error) {}
func (l *noopLogger) Close() error                         { return nil }
