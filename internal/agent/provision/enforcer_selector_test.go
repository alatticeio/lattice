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

package provision

import (
	"testing"

	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
)

func TestSelectEnforcerMode_Community(t *testing.T) {
	logger := log.GetLogger("test")
	mode := SelectEnforcerMode(&config.Config{}, "community", logger)
	if mode != ModeIPTables {
		t.Errorf("expected ModeIPTables for community tier, got %v", mode)
	}
}

func TestSelectEnforcerMode_Pro_EBPFRequested(t *testing.T) {
	logger := log.GetLogger("test")
	cfg := &config.Config{EnforcerMode: "ebpf"}
	mode := SelectEnforcerMode(cfg, "pro", logger)
	// eBPF may or may not be available in CI; just verify it doesn't panic.
	if mode != ModeIPTables && mode != ModeEBPF {
		t.Errorf("unexpected mode: %v", mode)
	}
}

func TestEnforcerMode_String(t *testing.T) {
	tests := []struct {
		mode EnforcerMode
		want string
	}{
		{ModeIPTables, "iptables"},
		{ModeUnset, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("EnforcerMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
