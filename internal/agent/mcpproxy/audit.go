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

package mcpproxy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	AuditLogPath = "/tmp/lattice-mcp-audit.jsonl"

	verdictAllow = "allow"
	verdictDeny  = "deny"
)

// MCPAuditEvent records a single MCP tool call policy decision.
type MCPAuditEvent struct {
	Timestamp    string `json:"timestamp"`
	AgentName    string `json:"agentName"`
	MCPServer    string `json:"mcpServer"`
	Tool         string `json:"tool"`
	ParamSummary string `json:"paramSummary,omitempty"`
	Verdict      string `json:"verdict"` // "allow" | "deny"
	DenyReason   string `json:"denyReason,omitempty"`
}

// AuditWriter writes MCPAuditEvents as JSONL to a file.
type AuditWriter struct {
	mu sync.Mutex
	f  *os.File
}

// NewAuditWriter opens (or creates) the JSONL audit log file.
func NewAuditWriter(path string) (*AuditWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log %s: %w", path, err)
	}
	return &AuditWriter{f: f}, nil
}

// Write appends one event to the JSONL file. Thread-safe.
func (w *AuditWriter) Write(event MCPAuditEvent) {
	if w == nil {
		return
	}
	event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	w.mu.Lock()
	_, _ = fmt.Fprintf(w.f, "%s\n", data)
	w.mu.Unlock()
}

// Close flushes and closes the audit file.
func (w *AuditWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

// summarizeParams produces a short, redacted summary of MCP tool params.
// Sensitive keys (password, token, secret, key, auth) are replaced with [REDACTED].
// Strings longer than 200 bytes are truncated.
func summarizeParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return ""
	}
	var parts []string
	sensitiveKeys := map[string]bool{
		"password": true, "token": true, "secret": true, "key": true, "auth": true,
	}
	for k, v := range params {
		kl := strings.ToLower(k)
		for sk := range sensitiveKeys {
			if strings.Contains(kl, sk) {
				parts = append(parts, k+"=[REDACTED]")
				goto next
			}
		}
		switch vt := v.(type) {
		case string:
			if len(vt) > 200 {
				parts = append(parts, fmt.Sprintf("%s=%s...[truncated, total=%dB]", k, vt[:100], len(vt)))
			} else {
				parts = append(parts, fmt.Sprintf("%s=%s", k, vt))
			}
		default:
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	next:
	}
	return strings.Join(parts, " ")
}
