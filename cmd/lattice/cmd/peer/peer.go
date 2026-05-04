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

// Package peer provides CLI commands for LatticePeer management.
package peer

import (
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/client"
	"github.com/alatticeio/lattice/internal/agent/config"
	"strings"

	"github.com/spf13/cobra"
)

// NewPeerCommand returns the top-level "peer" command.
func NewPeerCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "peer <sub-command>",
		Short: "Manage LatticePeer resources",
		Long: `Inspect and manage peers (agents) that have joined a workspace.

Peers are automatically created when an agent connects with a valid enrollment
token. Use 'peer label' to attach custom labels that policy selectors can match.`,
		Args: cobra.MinimumNArgs(1),
	}
	c.AddCommand(
		peerListCmd(),
		peerLabelCmd(),
	)
	return c
}

func newClient() (*cmd.Client, error) {
	return cmd.NewClient(config.Conf.ServerUrl, config.Conf.AuthToken)
}

// peerListCmd: lattice peer list -n <namespace>
func peerListCmd() *cobra.Command {
	var namespace string
	c := &cobra.Command{
		Use:     "list",
		Short:   "List peers in a workspace",
		Aliases: []string{"ls"},
		Example: `  lattice peer list -n wf-550e8400-e29b-41d4-a716-446655440000`,
		RunE: func(c *cobra.Command, args []string) error {
			if namespace == "" {
				return fmt.Errorf("namespace is required (-n <namespace>)\n  run 'lattice workspace list' to see available namespaces")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			return client.ListPeers(namespace)
		},
	}
	c.Flags().StringVarP(&namespace, "namespace", "n", "", "workspace namespace (required)")
	return c
}

// peerLabelCmd: lattice peer label <peer-name> -n <namespace> key=value [key=value...]
func peerLabelCmd() *cobra.Command {
	var namespace string
	c := &cobra.Command{
		Use:   "label <peer-name> key=value [key=value...]",
		Short: "Add or update labels on a LatticePeer",
		Long: `Merge one or more key=value labels into a LatticePeer's metadata.labels.

Labels are used by LatticePolicy PeerSelectors to target specific peers.
The peer controller automatically assigns alattice.io/network-{name}=true,
but you can add your own labels for fine-grained policy control.`,
		Example: `  # label a peer for a custom policy selector
  lattice peer label my-peer-abc123 -n wf-550e8400 env=prod role=gateway

  # list peers first to find the peer name
  lattice peer list -n wf-550e8400`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			if namespace == "" {
				return fmt.Errorf("namespace is required (-n <namespace>)")
			}
			peerName := args[0]
			labels := make(map[string]string)
			for _, kv := range args[1:] {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 || parts[0] == "" {
					return fmt.Errorf("invalid label format %q — expected key=value", kv)
				}
				labels[parts[0]] = parts[1]
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			return client.PeerLabel(namespace, peerName, labels)
		},
	}
	c.Flags().StringVarP(&namespace, "namespace", "n", "", "workspace namespace (required)")
	return c
}
