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
// In community builds this always returns ModeIPTables.
// In pro builds it checks kernel BPF capability and license validity.
func SelectEnforcerMode(logger *log.Logger) EnforcerMode {
	mode := selectEBPFAvailable()
	if mode == ModeEBPF {
		logger.Info("policy enforcement backend: eBPF")
		return ModeEBPF
	}
	logger.Info("policy enforcement backend: iptables")
	return ModeIPTables
}
