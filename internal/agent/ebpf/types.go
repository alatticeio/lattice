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

// TCActions represent actions that can be taken by the TC eBPF program.
const (
	ActionAccept = 1
	ActionDrop   = 0
)

// PolicyEntry represents a single policy rule for the eBPF map.
type PolicyEntry struct {
	SrcIP    string // CIDR or IP
	Protocol string // tcp, udp, or empty for all
	Port     int    // 0 for all ports
	Action   uint8  // ActionAccept or ActionDrop
}
