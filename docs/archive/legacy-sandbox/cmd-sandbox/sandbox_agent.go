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
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	agentName      string
	agentServerURL string
	agentToken     string
	agentReadyWait time.Duration
)

// agentCmd returns the `lattice sandbox agent` cobra command.
// This command is kept for manual debugging/testing. In production gVisor mode,
// Phase 1 (registration + wg0) runs on the pod kernel via bootstrapAgent, and
// Phase 2 runs the AI agent directly as PID 1 inside runsc.
func agentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Container PID 1: set up overlay network then exec AI agent (internal)",
		RunE:  runAgent,
	}
	cmd.Flags().StringVar(&agentName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&agentServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&agentToken, "token", "", "Enrollment token (required)")
	cmd.Flags().DurationVar(&agentReadyWait, "ready-wait", 3*time.Second, "Time to wait for WireGuard peers before exec-ing AI agent")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func runAgent(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing agent binary: pass it after '--', e.g.: lattice sandbox agent ... -- /path/to/agent [args]")
	}
	agentBinary := args[0]
	agentBinArgs := args[1:]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentconfig.Conf.AppId = agentName
	agentconfig.Conf.ServerUrl = agentServerURL
	agentconfig.Conf.WgPort = 51820

	// gVisor's devtmpfs may not auto-create /dev/net/ when --network=host
	// is used. Create the directory so wireguard-go can open /dev/net/tun.
	_ = os.MkdirAll("/dev/net", 0o755)

	var privKey wgtypes.Key
	var currentPeer *infra.Peer

	// Attempt to resume from persisted credentials (container restart path).
	if creds, loadErr := loadSandboxCredentials(); loadErr == nil {
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			privKey = key
			resumed, resumeErr := agent.ResumeSandboxViaNATS(ctx, agentServerURL, creds.JWT, agentName, key)
			if resumeErr == nil {
				currentPeer = resumed
				localIP := ""
				if currentPeer.Address != nil {
					localIP = *currentPeer.Address
				}
				fmt.Printf("[sandbox-agent] resumed %q, overlay IP=%s\n", agentName, localIP)
			} else {
				fmt.Printf("[sandbox-agent] resume failed (%v), registering fresh...\n", resumeErr)
			}
		}
	}

	if currentPeer == nil {
		var err error
		privKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("generate WireGuard key: %w", err)
		}
		fmt.Printf("[sandbox-agent] registering %q via NATS...\n", agentName)
		currentPeer, err = agent.RegisterSandboxViaNATS(ctx, agentServerURL, agentToken, agentName, privKey)
		if err != nil {
			return fmt.Errorf("sandbox registration failed: %w", err)
		}
		if saveErr := saveSandboxCredentials(privKey, currentPeer.Token); saveErr != nil {
			fmt.Printf("[sandbox-agent] warning: failed to persist credentials: %v\n", saveErr)
		}
	}

	localIP := ""
	if currentPeer.Address != nil {
		localIP = *currentPeer.Address
	}
	fmt.Printf("[sandbox-agent] overlay IP=%s\n", localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	logger := agentlog.GetLogger("sandbox-agent")
	agentJWT := currentPeer.Token

	// NewNode without CustomTUN: wireguard-go opens /dev/net/tun (gVisor
	// intercepts this and creates a virtual TUN interface in its netstack).
	// ProvisionerFactory is nil -> default kernel provisioner (iptables/eBPF);
	// gVisor intercepts iptables and netlink calls on the container's netns.
	nodeCfg := &agent.NodeConfig{
		Logger:      logger,
		Port:        51820,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomName:  agentName,
		CurrentPeer: currentPeer,
	}

	node, nodeErr := agent.NewNode(ctx, nodeCfg)
	if nodeErr != nil {
		return fmt.Errorf("create node: %w", nodeErr)
	}

	node.GetNetworkMap = func() (*infra.Message, error) {
		msg, getErr := node.GetNetMap(agentJWT)
		if getErr != nil {
			logger.Error("get network map failed", getErr)
			return nil, getErr
		}
		return msg, nil
	}

	if err := node.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}

	go node.StartHeartbeat(ctx)

	// Wait for WireGuard to establish peer sessions before exec-ing the agent.
	fmt.Printf("[sandbox-agent] waiting %s for WireGuard peers...\n", agentReadyWait)
	select {
	case <-time.After(agentReadyWait):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Drop ambient capabilities so the exec'd AI agent inherits zero privileges.
	// In gVisor, CAP_NET_ADMIN is virtualised; clearing the ambient set ensures
	// the AI agent process cannot manipulate network interfaces.
	if err := unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_CLEAR_ALL, 0, 0, 0); err != nil {
		// Non-fatal: log and continue. On some kernels/gVisor versions this may
		// return EINVAL if ambient capabilities are not supported.
		fmt.Printf("[sandbox-agent] warning: clear ambient caps: %v\n", err)
	}

	fmt.Printf("[sandbox-agent] exec %s %v\n", agentBinary, agentBinArgs)
	// syscall.Exec replaces this process image. On success it does not return.
	return syscall.Exec(agentBinary, append([]string{agentBinary}, agentBinArgs...), os.Environ())
}
