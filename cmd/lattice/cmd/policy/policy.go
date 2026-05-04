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

// Package policy provides CLI commands for network policy management.
package policy

import (
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/client"
	"github.com/alatticeio/lattice/internal/agent/config"

	"github.com/spf13/cobra"
)

// NewPolicyCommand returns the top-level "policy" command.
func NewPolicyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "policy <sub-command>",
		Short: "Manage network policies",
		Long: `Network policies control which peers can communicate with each other.
Lattice enforces default-deny: connected agents cannot exchange traffic
until an ALLOW policy is explicitly created for their workspace.`,
		Args: cobra.MinimumNArgs(1),
	}
	c.AddCommand(
		policyAddCmd(),
		policyAllowAllCmd(),
		policyRemoveCmd(),
		policyListCmd(),
	)
	return c
}

func newClient() (*cmd.Client, error) {
	return cmd.NewClient(config.Conf.ServerUrl, config.Conf.AuthToken)
}

// policyAddCmd: lattice policy add <name> -n <namespace> [flags]
func policyAddCmd() *cobra.Command {
	var namespace, action, description string
	c := &cobra.Command{
		Use:   "add <name>",
		Short: "Create or update a network policy",
		Long: `Create or update a network policy in a workspace.
Action can be ALLOW or DENY (default: ALLOW).
Empty ingress/egress rules mean "match all peers and all ports".`,
		Example: `  # allow all traffic in a workspace
  lattice policy add allow-all -n <namespace> --action ALLOW

  # deny a specific policy
  lattice policy add block-egress -n <namespace> --action DENY --desc "block outbound"`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if namespace == "" {
				return fmt.Errorf("namespace is required (-n <namespace>)\n  run 'lattice workspace list' to see available namespaces")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			return client.AddPolicy(namespace, args[0], action, description)
		},
	}
	c.Flags().StringVarP(&namespace, "namespace", "n", "", "workspace namespace (required)")
	c.Flags().StringVar(&action, "action", "ALLOW", "policy action: ALLOW or DENY")
	c.Flags().StringVar(&description, "desc", "", "human-readable description")
	return c
}

// policyAllowAllCmd: lattice policy allow-all -n <namespace>
func policyAllowAllCmd() *cobra.Command {
	var namespace string
	c := &cobra.Command{
		Use:   "allow-all",
		Short: "Allow all traffic between peers in a workspace (quickstart helper)",
		Long: `Create an allow-all policy that permits full-mesh communication
between every peer in the workspace. Ideal for development and single-tenant setups.

For production, replace this with fine-grained rules via the Dashboard.`,
		Example: `  lattice policy allow-all -n wf-550e8400-e29b-41d4-a716-446655440000`,
		RunE: func(c *cobra.Command, args []string) error {
			if namespace == "" {
				return fmt.Errorf("namespace is required (-n <namespace>)\n  run 'lattice workspace list' to see available namespaces")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			return client.AllowAll(namespace)
		},
	}
	c.Flags().StringVarP(&namespace, "namespace", "n", "", "workspace namespace (required)")
	return c
}

// policyRemoveCmd: lattice policy remove <name> -n <namespace>
func policyRemoveCmd() *cobra.Command {
	var namespace string
	c := &cobra.Command{
		Use:     "remove <name>",
		Short:   "Delete a policy by name",
		Aliases: []string{"rm", "delete"},
		Example: `  lattice policy remove allow-all -n wf-550e8400-e29b-41d4-a716-446655440000`,
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if namespace == "" {
				return fmt.Errorf("namespace is required (-n <namespace>)")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			return client.RemovePolicy(namespace, args[0])
		},
	}
	c.Flags().StringVarP(&namespace, "namespace", "n", "", "workspace namespace (required)")
	return c
}

// policyListCmd: lattice policy list -n <namespace>
func policyListCmd() *cobra.Command {
	var namespace string
	c := &cobra.Command{
		Use:     "list",
		Short:   "List policies in a workspace",
		Aliases: []string{"ls"},
		Example: `  lattice policy list -n wf-550e8400-e29b-41d4-a716-446655440000`,
		RunE: func(c *cobra.Command, args []string) error {
			if namespace == "" {
				return fmt.Errorf("namespace is required (-n <namespace>)")
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			return client.ListPolicies(namespace)
		},
	}
	c.Flags().StringVarP(&namespace, "namespace", "n", "", "workspace namespace (required)")
	return c
}
