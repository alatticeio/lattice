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

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/alatticeio/lattice/pkg/version"
)

// Client is the Lattice CLI HTTP client. It talks to the Lattice HTTP API
// using a Bearer JWT stored in the local config file.
type Client struct {
	serverURL string
	authToken string
	http      *http.Client
}

// NewClient constructs an HTTP client for CLI management commands.
func NewClient(serverURL, authToken string) (*Client, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("server-url is not configured. Run 'lattice init' first")
	}
	if authToken == "" {
		return nil, fmt.Errorf("not logged in. Run 'lattice login' first")
	}
	return &Client{
		serverURL: serverURL,
		authToken: authToken,
		http:      &http.Client{},
	}, nil
}

// do performs an authenticated HTTP request and decodes the JSON response body
// into result (pass nil to ignore the body). wsID is set as X-Workspace-Id
// when non-empty.
func (c *Client) do(ctx context.Context, method, path string, wsID string, body any, result any) error {
	var reqBody io.Reader
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(bs)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.serverURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if wsID != "" {
		req.Header.Set("X-Workspace-Id", wsID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("session expired. Run 'lattice login' to re-authenticate")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(raw))
	}

	if result != nil {
		// Unwrap {"code":0,"data":{...}} envelope
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return fmt.Errorf("unexpected response: %w", err)
		}
		if err := json.Unmarshal(envelope.Data, result); err != nil {
			return fmt.Errorf("decoding response data: %w", err)
		}
	}

	return nil
}

// Info prints client and server version information.
func (c *Client) Info(ctx context.Context) error {
	clientInfo := version.Get()
	fmt.Printf("AgentInterface Version: %s\n", clientInfo.Version)
	fmt.Printf("AgentInterface GitCommit: %s\n", clientInfo.GitCommit)

	var info struct {
		Version   string `json:"version"`
		GitCommit string `json:"gitCommit"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/info", "", nil, &info); err != nil {
		fmt.Println("Server Version: [Offline/Unknown]")
		return nil
	}
	fmt.Printf("Server Version: %s\n", info.Version)
	fmt.Printf("Server GitCommit: %s\n", info.GitCommit)
	return nil
}
