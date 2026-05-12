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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	sandboxName      string
	sandboxMode      string
	sandboxLocalIP   string
	sandboxWGEnabled bool
	sandboxServerURL string
	sandboxToken     string
)

func init() {
	startCmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	startCmd.Flags().StringVar(&sandboxMode, "mode", "gvisor", "Sandbox isolation mode: gvisor")
	startCmd.Flags().StringVar(&sandboxLocalIP, "local-ip", "", "VPN IP address (auto-registers if empty with --server-url)")
	startCmd.Flags().BoolVar(&sandboxWGEnabled, "wg", false, "Enable WireGuard tunnel attachment")
	startCmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL (for auto-registration)")
	startCmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token for auto-registration")

	_ = startCmd.MarkFlagRequired("name")
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a sandboxed agent environment",
	Long: `Start creates a gVisor-based network sandbox and optionally attaches
it to a WireGuard tunnel.

Examples:

  # Start with auto-registration to a Lattice control plane:
  lattice-agent-sandbox start --name agent-1 --server-url http://localhost:8080 --token lt-xxx

  # Start with pre-allocated IP (skip registration):
  lattice-agent-sandbox start --name agent-1 --local-ip 10.100.0.5`,
	RunE: runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	if sandboxLocalIP == "" && sandboxServerURL == "" {
		return fmt.Errorf("either --local-ip or --server-url (with --token) is required")
	}

	agentJWT := ""

	// Auto-register with control plane if server URL and token are provided.
	if sandboxServerURL != "" && sandboxToken != "" {
		regResp, err := registerWithServer(sandboxServerURL, sandboxToken, sandboxName, sandboxMode)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		if sandboxLocalIP == "" {
			sandboxLocalIP = regResp.LocalIP
		}
		agentJWT = regResp.JWT
		fmt.Printf("Registered agent %q with server %s\n", sandboxName, sandboxServerURL)
	}

	fmt.Printf("Starting sandbox %q, localIP=%s, wg=%v\n", sandboxName, sandboxLocalIP, sandboxWGEnabled)

	// Create the gVisor sandbox. In community builds this returns a
	// "Pro-only" error.
	sb, err := createSandbox(sandboxName, sandboxLocalIP, agentJWT)
	if err != nil {
		return fmt.Errorf("create sandbox: %w", err)
	}
	defer sb.Close()

	fmt.Printf("Sandbox %q ready (JWT: %v)\n", sandboxName, agentJWT != "")
	log.Printf("[sandbox] %s listening on %s", sandboxName, sandboxLocalIP)

	// Block until SIGINT or SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nShutting down...")
	return nil
}

// registerResponse is the JSON returned by POST /api/v1/agent-isolation/register.
type registerResponse struct {
	JWT               string `json:"JWT"`
	AgentIdentityName string `json:"AgentIdentityName"`
	LocalIP           string `json:"localIP"`
}

// registerWithServer calls the control plane registration API and returns
// the agent JWT and allocated IP.
func registerWithServer(serverURL, token, agentName, sandboxMode string) (*registerResponse, error) {
	// Generate a WireGuard key pair.
	rawKey := make([]byte, 32)
	if _, err := rand.Read(rawKey); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	pubKey := hex.EncodeToString(rawKey)

	body := map[string]string{
		"enrollmentToken": token,
		"agentName":       agentName,
		"publicKey":       pubKey,
		"sandbox":         sandboxMode,
	}
	payload, _ := json.Marshal(body)

	resp, err := http.Post(
		serverURL+"/api/v1/agent-isolation/register",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data registerResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w (body=%s)", err, string(respBody))
	}
	return &result.Data, nil
}
