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

package tunnel

// used for cli flags
var ServerUrl string
var SignalUrl string
var WrrpUrl string
var ShowNetLog bool

const (
	DefaultMTU = 1280
	// ConsoleDomain domain for service
	ConsoleDomain         = "http://console.alattice.io"
	ManagementDomain      = "console.alattice.io"
	SignalingDomain       = "signaling.alattice.io"
	TurnServerDomain      = "stun.alattice.io"
	DefaultManagementPort = 6060
	DefaultSignalingPort  = 4222
	DEFAULT_WRRP_PORT     = 6266
	DefaultTurnServerPort = 3478
)
