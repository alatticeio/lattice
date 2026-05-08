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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// configField describes the validation state of a single configuration item.
type configField struct {
	name       string
	value      string
	status     string // "OK" | "MISSING" | "DEFAULT"
	suggestion string
}

// ValidateAndReport is an environment-aware pre-start validation function.
//
// isServer=true (latticed server mode):
//   - If SignalingURL is empty, it is automatically set to nats://127.0.0.1:4222 and an Info log is printed.
//   - If DatabaseDSN is empty, it gracefully falls back to lattice.db (SQLite) in the current directory.
//   - Skips ServerUrl / Token validation, always returns nil.
//
// isServer=false (lattice agent client mode):
//   - Strictly validates that ServerUrl and Token are both non-empty.
//   - TTY environment: prints a formatted diagnostic report to stderr before returning an error.
//   - Non-TTY (Docker/K8s/CI): returns a concise error string directly.
//
// It is recommended to call this explicitly at the RunE stage of each subcommand,
// rather than in PersistentPreRunE, to avoid interfering with read-only subcommands
// such as --help, --version, or completion.
func ValidateAndReport(cfg *Config, isServer bool) error {
	if isServer {
		return applyServerDefaults(cfg)
	}
	return runClientValidation(cfg)
}

// applyServerDefaults fills in missing configuration for server (All-in-One) mode, always returns nil.
func applyServerDefaults(cfg *Config) error {
	if cfg.SignalingURL == "" {
		cfg.SignalingURL = "nats://127.0.0.1:4222"
		log.Info("All-in-One: applied default signaling-url", "value", cfg.SignalingURL)
	}
	if cfg.Database.DSN == "" {
		wd, _ := os.Getwd()
		cfg.Database.DSN = filepath.Join(wd, "lattice.db")
		cfg.Database.Driver = "sqlite"
		log.Info("All-in-One: applied default database DSN", "dsn", cfg.Database.DSN)
	}
	return nil
}

// runClientValidation performs strict field validation for agent mode.
func runClientValidation(cfg *Config) error {
	fields := []configField{
		{name: "server-url", value: cfg.ServerUrl, suggestion: "--server-url http://<HOST>:8080"},
		{name: "token", value: cfg.Token, suggestion: "--token <TOKEN>"},
	}

	var missing []string
	for i := range fields {
		if fields[i].value == "" {
			fields[i].status = "MISSING"
			missing = append(missing, fields[i].name)
		} else {
			fields[i].status = "OK"
			fields[i].suggestion = "-"
		}
	}

	if len(missing) == 0 {
		return nil
	}

	//if isStderrTTY() {
	printDiagnostic(fields, missing)
	//}

	return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
}

// isStderrTTY reports whether stderr is connected to a terminal.
// nolint:unused
func isStderrTTY() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// ─── Pretty Print ──────────────────────────────────────────────────────────────

// nolint:unused
const boxWidth = 80 // box character width (excluding the ║ borders on each side)

// printDiagnostic uses a cleaner section-based layout and removes redundant borders.
func printDiagnostic(fields []configField, missing []string) {
	w := os.Stderr

	// 1. Title: using a bold or simple separator
	fmt.Fprintln(w, "\n--- LATTICE SETUP ASSISTANT (Agent Mode) ---")                                 //nolint:errcheck
	fmt.Fprintf(w, "Error: Required configuration is missing. [Config: %s]\n\n", GetConfigFilePath()) //nolint:errcheck

	// 2. Config status table: simple column alignment
	fmt.Fprintf(w, "%-20s %-12s %s\n", "COMPONENT", "STATUS", "SUGGESTION") //nolint:errcheck
	fmt.Fprintln(w, strings.Repeat("-", 60))                                //nolint:errcheck

	for _, f := range fields {
		statusStr := f.status
		// For MISSING status, attach a visual indicator
		if f.status == "MISSING" {
			statusStr = "[MISSING]"
		}
		fmt.Fprintf(w, "%-20s %-12s %s\n", f.name, statusStr, f.suggestion) //nolint:errcheck
	}

	// 3. Quick fix: directly provide a copy-paste command
	fmt.Fprintln(w, "\n QUICK FIX:")                                                      //nolint:errcheck
	fmt.Fprintln(w, "   Run the following command to initialize:")                        //nolint:errcheck
	fmt.Fprintf(w, "   %s\n", "lattice up --server-url <API_URL> --token <TOKEN> --save") //nolint:errcheck

	// 4. Environment note: concise closing message
	fmt.Fprintln(w, "\n To use environment variables instead, check the documentation.") //nolint:errcheck
}
