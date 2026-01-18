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

// token cmd using for manager token
package token

import (
	"wireflow/internal/config"
	"wireflow/pkg/cmd"

	"github.com/spf13/cobra"
)

// NewTokenCommand create token cmd.
func NewTokenCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "token <sub-command>",
		Short: "",
		Long:  `create a token, peer using token to join network`,
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(tokenCreateCmd())

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
		Example: `   wireflow token create dev-team
  
  # set token limit and expiry time
wireflow token create dev-team --limit 5 --expiry 168h -n wireflow-system`,
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
	client, err := cmd.NewClient(config.Conf.SignalingURL)
	if err != nil {
		return err
	}

	err = client.CreateToken(namespace, name, expiry)

	if err != nil {
		return err
	}
	return nil
}
