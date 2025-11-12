// Copyright 2025 Wireflow.io, Inc.
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

package internal

// ConfigureManager is the interface for configuring WireGuard interfaces.
type ConfigureManager interface {
	// ConfigureWG configures the WireGuard interface.
	ConfigureWG() error

	AddPeer(peer *SetPeer) error

	GetAddress() string

	GetIfaceName() string

	GetPeersManager() *NodeManager

	RemovePeer(peer *SetPeer) error
	//
	//AddAllowedIPs(peer *SetPeer) error
	//
	//RemoveAllowedIPs(peer *SetPeer) error
}
