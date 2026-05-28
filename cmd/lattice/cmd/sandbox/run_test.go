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

package sandbox_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/alatticeio/lattice/cmd/lattice/cmd/sandbox"
)

func TestSandboxCmd_RunSubcommandRegistered(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("'run' subcommand requires Linux (netstack + iptables)")
	}
	cmd := sandbox.SandboxCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "run" || strings.HasPrefix(sub.Use, "run ") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'run' subcommand to be registered on sandbox command")
	}
}

func TestSandboxCmd_SidecarSubcommandRegistered(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("'sidecar' subcommand requires Linux (netstack + iptables)")
	}
	cmd := sandbox.SandboxCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "sidecar" || strings.HasPrefix(sub.Use, "sidecar ") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'sidecar' subcommand to be registered on sandbox command")
	}
}
