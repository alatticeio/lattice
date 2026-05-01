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
	"github.com/alatticeio/lattice/internal/agent/ebpf"
	"github.com/alatticeio/lattice/internal/agent/log"
)

// NewEBPFEnforcer creates an eBPF-based PolicyEnforcer.
// In community builds this falls back to iptables since the eBPF manager
// is a stub that always returns errors.
func NewEBPFEnforcer(iface string, logger *log.Logger) PolicyEnforcer {
	mgr := ebpf.NewManager(iface, logger)
	if err := mgr.Load(); err != nil {
		logger.Warn("eBPF load failed, falling back to iptables", "err", err)
		return NewIptablesEnforcer(logger, iface)
	}
	return mgr
}
