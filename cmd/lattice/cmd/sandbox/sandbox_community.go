//go:build !pro

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

package sandbox

import (
	"errors"

	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a sandboxed agent environment (Pro only)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("lattice sandbox is a Lattice Pro feature")
		},
	}
	registerStartFlags(cmd)
	return cmd
}

func registerStartFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL")
	cmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token")
	cmd.Flags().StringVar(&sandboxProxyAddr, "proxy-addr", "", "HTTP forward proxy listen address")
	cmd.Flags().StringArrayVar(&sandboxForwardRules, "forward", nil, "Inbound forward rule: overlayPort:targetAddr")
	cmd.Flags().StringVar(&sandboxEgressAllow, "egress-allow", "", "Comma-separated allowed egress CIDRs")
	cmd.Flags().BoolVar(&sandboxEgressDeny, "egress-default-deny", false, "Whitelist egress mode")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
}
