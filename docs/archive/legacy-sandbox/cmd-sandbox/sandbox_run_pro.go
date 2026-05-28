//go:build pro && linux

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
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var sandboxRunReadyTimeout time.Duration

// registerRunCmd adds `lattice sandbox run` to the sandbox parent command.
func registerRunCmd(parent *cobra.Command) {
	parent.AddCommand(runCmd())
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run an AI agent inside a Lattice sandbox (Pro)",
		Long: `Run starts a pod-mode network sandbox, injects ALL_PROXY into the
child process environment, and executes the given command. When the child
process exits, the sandbox is automatically cleaned up.

The child process can use any standard HTTP/HTTPS client that respects
ALL_PROXY (curl, Python requests/httpx, Node.js fetch, Go net/http, etc.)
to route traffic through the Lattice overlay network without any code changes.

Examples:

  # Run a Python agent through the sandbox:
  lattice sandbox run --name my-agent --server-url http://latticed:8080 --token lt-xxx \
    -- python agent.py --task "analyze data"

  # Run Claude CLI through the sandbox:
  lattice sandbox run --name my-agent --server-url http://latticed:8080 --token lt-xxx \
    -- claude --model claude-opus-4-6

  # Restrict egress to the overlay subnet only:
  lattice sandbox run --name my-agent --server-url http://latticed:8080 --token lt-xxx \
    --egress-allow 10.0.0.0/8 --egress-default-deny \
    -- python agent.py`,
		RunE: runRun,
		Args: cobra.ArbitraryArgs,
	}

	cmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token (required)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")

	cmd.Flags().StringVar(&sandboxProxyAddr, "proxy-addr", "",
		"SOCKS5 proxy listen address; empty picks a random port")
	cmd.Flags().StringVar(&sandboxEgressAllow, "egress-allow", "", "Comma-separated allowed egress CIDRs")
	cmd.Flags().BoolVar(&sandboxEgressDeny, "egress-default-deny", false, "Whitelist egress mode")
	cmd.Flags().StringArrayVar(&sandboxForwardRules, "forward", nil, "Inbound forward rule: overlayPort:targetAddr")
	cmd.Flags().DurationVar(&sandboxRunReadyTimeout, "ready-timeout", 30*time.Second,
		"Maximum time to wait for sandbox to be ready")
	cmd.Flags().IntVar(&sandboxWgPort, "wg-port", 0,
		"WireGuard UDP listen port (0 = random, use when port 51820 is already in use)")

	cmd.Flags().StringVar(&sandboxMode, "mode", "pod", "Isolation mode: pod | gvisor")
	cmd.Flags().StringVar(&sandboxAgentRootFS, "agent-rootfs", "", "Root filesystem path for runsc container (gvisor mode)")
	cmd.Flags().StringVar(&sandboxAgentBinary, "agent-binary", "", "Agent entrypoint binary (gvisor mode)")
	cmd.Flags().StringSliceVar(&sandboxAgentArgs, "agent-args", nil, "Agent entrypoint arguments (gvisor mode)")

	return cmd
}

func runRun(_ *cobra.Command, args []string) error {
	if sandboxMode == "gvisor" {
		return runRunGVisor(args)
	}
	return runRunPod(args)
}

func runRunPod(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command: use -- <command> [args...], e.g.: lattice sandbox run ... -- python agent.py")
	}

	// Pre-allocate a local port for SOCKS5 so we know the address before the
	// driver starts (avoids needing an Addr() method on the SOCKS5 server).
	proxyAddr := sandboxProxyAddr
	if proxyAddr == "" {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("allocate proxy port: %w", err)
		}
		proxyAddr = ln.Addr().String()
		ln.Close()
	}

	readyCh := make(chan struct{}, 1)
	cfg := DriverConfig{
		SandboxName:  sandboxName,
		ServerURL:    sandboxServerURL,
		Token:        sandboxToken,
		ProxyAddr:    proxyAddr,
		EgressAllow:  sandboxEgressAllow,
		EgressDeny:   sandboxEgressDeny,
		ForwardRules: sandboxForwardRules,
		WgPort:       sandboxWgPort,
		ReadyCh:      readyCh,
	}

	if err := validateDriverConfig("pod", cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	driver := NewPodDriver(cfg)
	driverDone := make(chan error, 1)
	go func() {
		driverDone <- driver.Start(ctx)
	}()

	// Wait for sandbox to be ready.
	select {
	case <-readyCh:
		// sandbox is up, SOCKS5 is listening
	case <-time.After(sandboxRunReadyTimeout):
		cancel()
		<-driverDone
		return fmt.Errorf("sandbox not ready after %s", sandboxRunReadyTimeout)
	case err := <-driverDone:
		if err != nil {
			return fmt.Errorf("sandbox failed to start: %w", err)
		}
		return fmt.Errorf("sandbox exited before becoming ready")
	}

	// Build child environment with proxy injected.
	env := append(os.Environ(),
		"ALL_PROXY=socks5://"+proxyAddr,
		"all_proxy=socks5://"+proxyAddr,
		"LATTICE_SANDBOX_NAME="+sandboxName,
	)

	fmt.Printf("Executing: %v\n", args)

	child := exec.CommandContext(ctx, args[0], args[1:]...)
	child.Env = env
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	if err := child.Start(); err != nil {
		cancel()
		<-driverDone
		return fmt.Errorf("start child process: %w", err)
	}

	childDone := make(chan error, 1)
	go func() {
		childDone <- child.Wait()
	}()

	var childErr error
	select {
	case childErr = <-childDone:
		// Child exited normally (or with error) — cancel sandbox.
		cancel()
		<-driverDone
	case driverErr := <-driverDone:
		// Sandbox died unexpectedly — terminate child.
		fmt.Fprintf(os.Stderr, "sandbox terminated unexpectedly: %v\n", driverErr)
		_ = child.Process.Signal(syscall.SIGTERM)
		select {
		case <-childDone:
		case <-time.After(5 * time.Second):
			_ = child.Process.Kill()
			<-childDone
		}
		return fmt.Errorf("sandbox terminated unexpectedly: %w", driverErr)
	}

	// Propagate child's exit code.
	var exitErr *exec.ExitError
	if errors.As(childErr, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return childErr
}

func runRunGVisor(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command: use -- <command> [args...]")
	}

	cfg := DriverConfig{
		SandboxName: sandboxName,
		ServerURL:   sandboxServerURL,
		Token:       sandboxToken,
		RootFS:      sandboxAgentRootFS,
		AgentBinary: sandboxAgentBinary,
		AgentArgs:   sandboxAgentArgs,
	}

	// If --agent-binary not set, use the first arg as the agent binary.
	if cfg.AgentBinary == "" {
		cfg.AgentBinary = args[0]
		cfg.AgentArgs = args[1:]
	}

	if err := validateDriverConfig("gvisor", cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	driver := NewRunscDriver(cfg)
	fmt.Printf("Starting sandbox %q in gvisor mode...\n", sandboxName)
	return driver.Start(ctx)
}
