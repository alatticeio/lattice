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

package ebpf

import (
	"github.com/alatticeio/lattice/internal/agent/infra"
)

// PolicyEnforcer is the interface that the eBPF manager implements.
// It mirrors provision.PolicyEnforcer so that the eBPF Manager can be
// used as a drop-in replacement for the iptables ruleProvisioner.
type PolicyEnforcer interface {
	Name() string
	Provision(rule *infra.FirewallRule) error
	Cleanup() error
	SetupNAT(interfaceName string) error
}
