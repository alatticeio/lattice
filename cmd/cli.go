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

package cmd

import "github.com/spf13/cobra"

func cli() *cobra.Command {
	return &cobra.Command{
		Short:        "any",
		Use:          "any [command]",
		SilenceUsage: true,
		Long:         `start controller`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCli()
		},
	}
}

func runCli() error {
	return nil
}
