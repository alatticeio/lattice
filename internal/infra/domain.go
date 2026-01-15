// Copyright 2025 The Wireflow Authors, Inc.
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

package infra

// used for cli flags
var ServerUrl string
var SignalUrl string
var ShowNetLog bool

const (
	DefaultMTU = 1420
	// ConsoleDomain domain for service
	ConsoleDomain         = "http://console.wireflow.run"
	ManagementDomain      = "console.wireflow.run"
	SignalingDomain       = "signaling.wireflow.run"
	TurnServerDomain      = "stun.wireflow.run"
	DefaultManagementPort = 6060
	DefaultSignalingPort  = 4222
	DefaultTurnServerPort = 3478
)
