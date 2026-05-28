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

import "github.com/spf13/cobra"

// SandboxCmd returns the top-level `sandbox` cobra command.
func SandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage AI agent sandboxes",
		Long: `The sandbox commands start and configure AI agent sandboxes on the
Lattice overlay network.

All sandbox traffic flows through a kernel WireGuard interface (wg0) created
in the container's network namespace.

Scenarios:
  sandbox run    — single container, fork + exec with WireGuard overlay
  sandbox sidecar — Kubernetes sidecar with transparent proxy
  sandbox init   — Kubernetes init container, writes iptables rules and exits`,
	}
	addRunCmd(cmd)
	addSidecarCmd(cmd)
	addInitCmd(cmd)
	return cmd
}
