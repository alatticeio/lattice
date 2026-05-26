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
	"fmt"
	"os"
	"strings"

	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Configure Lattice connection parameters and save to config file",
		Long: `Prompt for required connection parameters and save them to
~/.lattice/lattice.yaml. After init, run "lattice up" with no flags.`,
		Example: `  lattice init
  lattice init --server https://lattice.company.com --token lt-enroll-xxx
  lattice init --config-dir /etc/lattice`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd)
		},
	}
	cmd.Flags().String("server", "", "Management server URL (non-interactive)")
	cmd.Flags().String("token", "", "Enrollment token (non-interactive)")
	return cmd
}

func runInit(cmd *cobra.Command) error {
	cfgPath := config.GetConfigFilePath()
	v := cfgManager.Viper()

	// Non-interactive mode: --server and --token both provided via flags
	serverFlag, _ := cmd.Flags().GetString("server")
	tokenFlag, _ := cmd.Flags().GetString("token")
	if serverFlag != "" && tokenFlag != "" {
		// Non-interactive mode: silently overwrite any existing config.
		// This is intentional — callers (e.g. demo enrollment scripts) expect automation-friendly behaviour.
		if _, err := os.Stat(cfgPath); err == nil {
			fmt.Fprintf(os.Stderr, "Warning: overwriting existing config at %s\n", cfgPath)
		}
		v.Set("server-url", serverFlag)
		v.Set("token", tokenFlag)
		if err := cfgManager.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("Config saved to %s\n", cfgPath)
		return nil
	}

	scanner := bufio.NewScanner(os.Stdin)

	// If the config file already exists, ask whether to overwrite it
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Printf("Config file already exists at %s\n", cfgPath)
		fmt.Print("Overwrite existing config? [y/N]: ")
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if !strings.EqualFold(answer, "y") {
			fmt.Println("Aborted. Existing config unchanged.")
			return nil
		}
	}

	// Required fields
	serverURL := prompt(scanner, "Management server URL (--server-url)", v.GetString("server-url"))
	token := prompt(scanner, "Enrollment token (--token)", v.GetString("token"))

	// Optional fields
	relayURL := promptOptional(scanner, "Relay TCP URL (--relay-url, optional)")
	relayQuicURL := promptOptional(scanner, "Relay QUIC URL (--relay-quic-url, optional)")

	v.Set("server-url", serverURL)
	v.Set("token", token)
	if relayURL != "" {
		v.Set("relay-url", relayURL)
	}
	if relayQuicURL != "" {
		v.Set("relay-quic-url", relayQuicURL)
	}

	if err := cfgManager.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\nConfig saved to %s\n", cfgPath)
	fmt.Println(`Next steps:
  lattice login   — authenticate for management commands (workspace, token, policy)
  lattice up      — connect this device as a peer`)
	return nil
}

// prompt prints the prompt and reads input; returns defaultVal if the user presses Enter.
func prompt(scanner *bufio.Scanner, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("? %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("? %s: ", label)
	}
	scanner.Scan()
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return defaultVal
	}
	return val
}

// promptOptional prints an optional prompt; returns an empty string if the user presses Enter.
func promptOptional(scanner *bufio.Scanner, label string) string {
	fmt.Printf("? %s (press Enter to skip): ", label)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}
