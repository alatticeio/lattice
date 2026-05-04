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

// token cmd using for manager token
package token

import (
	"github.com/alatticeio/lattice/internal/agent/client"
	"github.com/alatticeio/lattice/internal/agent/config"

	"github.com/spf13/cobra"
)

// NewTokenCommand returns the top-level "token" command.
func NewTokenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token <sub-command>",
		Short: "Manage enrollment tokens",
		Long:  `Tokens authorize agents to join a workspace. Agents use tokens during 'lattice up'.`,
		Args:  cobra.MinimumNArgs(1),
	}
	cmd.AddCommand(tokenCreateCmd(), tokenListCmd(), tokenRemoveCmd())
	return cmd
}

func tokenCreateCmd() *cobra.Command {
	var (
		limit             int
		namespace, expiry string
	)
	cmd := &cobra.Command{
		Use:   "create <token-name>",
		Short: "create a token",
		Long: `create a token for peer to join network。
    
params description:
  token-name    token name`,
		Example: `   lattice token create dev-team
  
  # set token limit and expiry time
lattice token create dev-team --limit 5 --expiry 168h -n lattice-system`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tokenName := args[0]

			return runCreate(namespace, tokenName, expiry)

		},
	}

	fs := cmd.Flags()
	fs.StringVarP(&namespace, "namespace", "n", "", "namespace of token")
	fs.StringVarP(&expiry, "expiry", "e", "", "token expiry time")
	fs.IntVarP(&limit, "limit", "l", 0, "token limit")

	return cmd
}

func runCreate(namespace, name, expiry string) error {
	client, err := cmd.NewClient(config.Conf.ServerUrl, config.Conf.AuthToken)
	if err != nil {
		return err
	}
	return client.CreateToken(namespace, name, expiry)
}

// tokenListCmd: lattice token list [-n <namespace>]
func tokenListCmd() *cobra.Command {
	var namespace string
	c := &cobra.Command{
		Use:     "list",
		Short:   "List enrollment tokens",
		Aliases: []string{"ls"},
		Example: `  # all tokens
  lattice token list

  # tokens in a specific workspace
  lattice token list -n wf-550e8400-e29b-41d4-a716-446655440000`,
		RunE: func(c *cobra.Command, args []string) error {
			client, err := cmd.NewClient(config.Conf.ServerUrl, config.Conf.AuthToken)
			if err != nil {
				return err
			}
			return client.ListTokens(namespace)
		},
	}
	c.Flags().StringVarP(&namespace, "namespace", "n", "", "filter by workspace namespace")
	return c
}

// tokenRemoveCmd: lattice token remove <token> -n <namespace>
func tokenRemoveCmd() *cobra.Command {
	var namespace string
	c := &cobra.Command{
		Use:     "remove <token>",
		Short:   "Revoke an enrollment token",
		Aliases: []string{"rm", "delete"},
		Example: `  lattice token remove dev-team -n lattice-system`,
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			client, err := cmd.NewClient(config.Conf.ServerUrl, config.Conf.AuthToken)
			if err != nil {
				return err
			}
			return client.RemoveToken(namespace, args[0])
		},
	}
	c.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace the token belongs to")
	_ = c.MarkFlagRequired("namespace")
	return c
}
