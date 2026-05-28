//go:build linux

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
	"testing"
)

func TestBuildIPTablesRules(t *testing.T) {
	rules := buildIPTablesRules(15001, 1337)

	// Must contain a REDIRECT rule on port 15001.
	foundRedirect := false
	foundSkipUID := false
	for _, r := range rules {
		args := r
		hasRedirect := false
		hasPort := false
		hasUID := false
		for i, a := range args {
			if a == "REDIRECT" {
				hasRedirect = true
			}
			if a == "--to-ports" && i+1 < len(args) && args[i+1] == "15001" {
				hasPort = true
			}
			if a == "--uid-owner" && i+1 < len(args) && args[i+1] == "1337" {
				hasUID = true
			}
		}
		if hasRedirect && hasPort {
			foundRedirect = true
		}
		if hasUID {
			foundSkipUID = true
		}
	}
	if !foundRedirect {
		t.Error("expected a REDIRECT rule on port 15001")
	}
	if !foundSkipUID {
		t.Error("expected a --uid-owner 1337 skip rule")
	}
}
