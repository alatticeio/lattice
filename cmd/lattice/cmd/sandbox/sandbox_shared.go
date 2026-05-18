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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	shimfwd "github.com/alatticeio/lattice-shim/shim"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// sandboxCredentials holds the persisted registration state for crash recovery.
type sandboxCredentials struct {
	PrivateKey string `json:"privateKey"`
	JWT        string `json:"jwt"`
}

func sandboxCredentialsFile() string {
	dir := os.Getenv("LATTICE_CONFIG_DIR")
	if dir == "" {
		dir = "/etc/lattice"
	}
	return filepath.Join(dir, "sandbox-credentials.json")
}

func loadSandboxCredentials() (*sandboxCredentials, error) {
	data, err := os.ReadFile(sandboxCredentialsFile())
	if err != nil {
		return nil, err
	}
	var creds sandboxCredentials
	if err = json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	if creds.PrivateKey == "" || creds.JWT == "" {
		return nil, fmt.Errorf("incomplete credentials")
	}
	return &creds, nil
}

func saveSandboxCredentials(privKey wgtypes.Key, jwt string) error {
	creds := sandboxCredentials{PrivateKey: privKey.String(), JWT: jwt}
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(sandboxCredentialsFile()), 0o755)
	return os.WriteFile(sandboxCredentialsFile(), data, 0o600)
}

// fileAuditWriter implements shimfwd.AuditWriter by appending JSON lines to a file.
type fileAuditWriter struct {
	mu sync.Mutex
	f  *os.File
}

func newFileAuditWriter(path string) (*fileAuditWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &fileAuditWriter{f: f}, nil
}

func (w *fileAuditWriter) Write(event shimfwd.AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = fmt.Fprintf(w.f, "%s\n", data)
	return err
}
