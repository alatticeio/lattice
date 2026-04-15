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

package cmd

import (
	"wireflow/agent"
	"wireflow/internal/config"

	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current agent status and connected peers",
		Long: `Display the WireGuard interface information and the status of all peers.

A peer is considered connected if a WireGuard handshake was completed within
the last 3 minutes. Traffic counters show bytes sent (↑) and received (↓)
since the interface was last started.

Example output:

  Interface : wg0
  Address   : 10.100.0.1/24
  Public Key: abc123...=
  Port      : 51820

  Peers: 2 total, 1 connected

    Peer      : xyz456...=
    Address   : 10.100.0.2/32
    Endpoint  : 203.0.113.1:51820
    Handshake : 12 seconds ago
    Traffic   : ↑ 1.2 MB  ↓ 3.4 MB
    Status    : connected`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return agent.Status(config.Conf)
		},
	}
}
