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
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
)

// EnforcerMode represents the selected policy enforcement backend.
type EnforcerMode int

const (
	ModeUnset EnforcerMode = iota
	ModeIPTables
	ModeEBPF
)

func (m EnforcerMode) String() string {
	switch m {
	case ModeIPTables:
		return "iptables"
	case ModeEBPF:
		return "ebpf"
	default:
		return "unknown"
	}
}

// SelectEnforcerMode decides which PolicyEnforcer backend to use.
// cfg.EnforcerMode may be "auto", "iptables", or "ebpf".
// "auto" defers to build-tag detection (community -> iptables, pro -> kernel probe).
// "ebpf" falls back to iptables with a warning if eBPF is unavailable.
func SelectEnforcerMode(cfg *config.Config, logger *log.Logger) EnforcerMode {
	switch cfg.EnforcerMode {
	case "iptables":
		logger.Info("policy enforcement backend: iptables (source: explicit)")
		return ModeIPTables
	case "ebpf":
		if mode := selectEBPFAvailable(); mode == ModeEBPF {
			logger.Info("policy enforcement backend: eBPF (source: explicit)")
			return ModeEBPF
		}
		logger.Warn("ebpf requested but unavailable, falling back to iptables")
		return ModeIPTables
	default: // "auto" or empty
		mode := selectEBPFAvailable()
		if mode == ModeEBPF {
			logger.Info("policy enforcement backend: eBPF (source: auto)")
			return ModeEBPF
		}
		logger.Info("policy enforcement backend: iptables (source: auto)")
		return ModeIPTables
	}
}
