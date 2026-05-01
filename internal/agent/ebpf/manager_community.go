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

//go:build !pro

package ebpf

import (
	"errors"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
)

var errEBPFNotAvailable = errors.New("eBPF policy enforcement is a Lattice Pro feature — upgrade at https://alattice.io/pro")

// Manager is a no-op stub in community builds.
type Manager struct {
	logger *log.Logger
}

// NewManager creates a stub manager that always returns errors.
func NewManager(iface string, logger *log.Logger) *Manager {
	return &Manager{logger: logger}
}

func (m *Manager) Load() error                           { return errEBPFNotAvailable }
func (m *Manager) Provision(_ *infra.FirewallRule) error { return errEBPFNotAvailable }
func (m *Manager) Cleanup() error                        { return nil }
func (m *Manager) Name() string                          { return "ebpf" }
func (m *Manager) SetupNAT(_ string) error               { return nil }
