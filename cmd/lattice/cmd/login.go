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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Lattice control plane",
		Long: `Log in to Lattice and save a session token for management commands
(workspace, token, policy). The session lasts 30 days.

Run this once after 'lattice init'. Re-run when the session expires.`,
		Example: `  lattice login`,
		RunE:    runLogin,
	}
}

func runLogin(_ *cobra.Command, _ []string) error {
	serverURL := config.Conf.ServerUrl
	if serverURL == "" {
		return fmt.Errorf("server-url is not configured. Run 'lattice init' first")
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Printf("Logging in to %s\n", serverURL)
	fmt.Print("Username: ")
	scanner.Scan()
	username := strings.TrimSpace(scanner.Text())

	fmt.Print("Password: ")
	var password string
	if term.IsTerminal(int(syscall.Stdin)) {
		pw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}
		password = string(pw)
	} else {
		scanner.Scan()
		password = strings.TrimSpace(scanner.Text())
	}

	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
		"client":   "cli",
	})

	resp, err := http.Post( //nolint:noctx
		serverURL+"/api/v1/users/login",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, string(raw))
	}

	var result struct {
		Data struct {
			Token string `json:"token"`
			User  string `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("unexpected response: %w", err)
	}
	if result.Data.Token == "" {
		return fmt.Errorf("server returned empty token")
	}

	// Persist JWT to config file
	cfgManager.Viper().Set("auth-token", result.Data.Token)
	if err := cfgManager.Save(); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	fmt.Printf("Logged in as %s. Session saved (expires in 30 days).\n", result.Data.User)
	return nil
}
