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

	"github.com/alatticeio/lattice/internal/agent/infra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestSandboxCredentialRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LATTICE_CONFIG_DIR", dir)

	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	jwt := "test-jwt-token"

	if err := saveSandboxCredentials(key, jwt); err != nil {
		t.Fatalf("saveSandboxCredentials: %v", err)
	}

	creds, err := loadSandboxCredentials()
	if err != nil {
		t.Fatalf("loadSandboxCredentials: %v", err)
	}
	if creds.PrivateKey != key.String() {
		t.Errorf("PrivateKey mismatch: got %q, want %q", creds.PrivateKey, key.String())
	}
	if creds.JWT != jwt {
		t.Errorf("JWT mismatch: got %q, want %q", creds.JWT, jwt)
	}
}

func TestOverlayAddr(t *testing.T) {
	if got := overlayAddr(nil); got != "" {
		t.Errorf("overlayAddr(nil) = %q, want empty", got)
	}

	peer := &infra.Peer{}
	if got := overlayAddr(peer); got != "" {
		t.Errorf("overlayAddr(peer without address) = %q, want empty", got)
	}

	addr := "10.0.0.1"
	peer = &infra.Peer{Address: &addr}
	if got := overlayAddr(peer); got != addr {
		t.Errorf("overlayAddr(peer with address) = %q, want %q", got, addr)
	}
}
