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

//go:build pro

package provision

import (
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
)

func selectEBPFAvailable() EnforcerMode {
	// Remove memlock rlimit (required for eBPF).
	if err := rlimit.RemoveMemlock(); err != nil {
		return ModeIPTables
	}
	// Probe basic eBPF support.
	if ok, _ := ebpf.HaveProgramType(ebpf.SchedCLS); !ok {
		return ModeIPTables
	}
	return ModeEBPF
}
