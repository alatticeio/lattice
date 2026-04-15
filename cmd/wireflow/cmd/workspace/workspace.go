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

// Package workspace provides CLI commands for workspace management.
package workspace

import (
	"wireflow/internal/config"
	"wireflow/pkg/cmd"

	"github.com/spf13/cobra"
)

// NewWorkspaceCommand returns the top-level "workspace" command.
func NewWorkspaceCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "workspace <sub-command>",
		Short: "Manage workspaces",
		Long: `Workspaces are isolated network environments.
Each workspace maps to a Kubernetes namespace and can contain
its own set of peers, tokens, and network policies.`,
		Args: cobra.MinimumNArgs(1),
	}
	c.AddCommand(
		workspaceAddCmd(),
		workspaceRemoveCmd(),
		workspaceListCmd(),
	)
	return c
}

func newClient() (*cmd.Client, error) {
	return cmd.NewClient(config.Conf.SignalingURL)
}

// workspaceAddCmd: wireflow workspace add <slug> [flags]
func workspaceAddCmd() *cobra.Command {
	var namespace, displayName string
	c := &cobra.Command{
		Use:   "add <slug>",
		Short: "Create a new workspace",
		Long: `Create a workspace. The slug is a short, URL-safe identifier (e.g. "dev", "prod").
The namespace is auto-generated from the workspace UUID if not provided.`,
		Example: `  # minimal
  wireflow workspace add dev

  # with display name
  wireflow workspace add dev --display-name "Development"

  # with explicit namespace
  wireflow workspace add dev -n wireflow-dev --display-name "Development"`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			return client.AddWorkspace(args[0], namespace, displayName)
		},
	}
	c.Flags().StringVarP(&namespace, "namespace", "n", "", "K8s namespace (auto-generated if omitted)")
	c.Flags().StringVar(&displayName, "display-name", "", "human-readable workspace name")
	return c
}

// workspaceRemoveCmd: wireflow workspace remove <namespace>
func workspaceRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <namespace>",
		Short:   "Delete a workspace",
		Long:    `Delete a workspace by its K8s namespace (shown in 'wireflow workspace list').`,
		Example: `  wireflow workspace remove wf-550e8400-e29b-41d4-a716-446655440000`,
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			return client.RemoveWorkspace(args[0])
		},
	}
}

// workspaceListCmd: wireflow workspace list
func workspaceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all workspaces",
		Aliases: []string{"ls"},
		RunE: func(c *cobra.Command, args []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			return client.ListWorkspaces()
		},
	}
}
