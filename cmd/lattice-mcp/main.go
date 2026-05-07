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

// lattice-mcp is an MCP stdio server that exposes Lattice network management
// tools to AI assistants such as Claude Desktop and Cursor.
//
// Usage:
//
//	lattice-mcp --workspace ws-xxx
//
// Claude Desktop configuration (~/.config/claude/claude_desktop_config.json):
//
//	{
//	  "mcpServers": {
//	    "lattice": {
//	      "command": "lattice-mcp",
//	      "args": ["--workspace", "YOUR_WORKSPACE_ID"]
//	    }
//	  }
//	}
package main

import (
	"fmt"
	"os"

	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/mcp"
	"github.com/alatticeio/lattice/pkg/version"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var workspaceID string

	cmd := &cobra.Command{
		Use:   "lattice-mcp",
		Short: "MCP server for Lattice network management",
		Long: `lattice-mcp exposes Lattice network management as an MCP server.
Connect it to Claude Desktop, Cursor, or any MCP-compatible AI assistant
to manage your WireGuard overlay network with natural language.

Run 'lattice login' first to authenticate, then add to Claude Desktop config:
  {
    "mcpServers": {
      "lattice": {
        "command": "lattice-mcp",
        "args": ["--workspace", "YOUR_WORKSPACE_ID"]
      }
    }
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, workspaceID)
		},
	}

	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Lattice workspace ID to scope all tool calls (required)")
	_ = cmd.MarkFlagRequired("workspace")

	return cmd
}

func run(cmd *cobra.Command, workspaceID string) error {
	if err := config.GetManager().Load(cmd); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg := config.GlobalConfig

	if cfg.ServerUrl == "" {
		return fmt.Errorf("server-url is required in lattice.yaml (run 'lattice login' to configure)")
	}
	if cfg.AuthToken == "" {
		return fmt.Errorf("auth-token is not set — run 'lattice login' first")
	}

	srv := mcp.NewServer(cfg.ServerUrl, cfg.AuthToken, mcp.ServerOptions{
		WorkspaceID: workspaceID,
		Version:     version.Version,
	})

	fmt.Fprintf(os.Stderr, "Lattice MCP server started (workspace: %s, server: %s)\n",
		workspaceID, cfg.ServerUrl)

	return srv.Run()
}
