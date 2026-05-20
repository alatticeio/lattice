//go:build pro

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
	"os/exec"
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

	// gVisor sandbox virtual eth0 configuration (static, no DHCP in K8s).
	agentSandboxIP      string
	agentSandboxGateway string
	agentSandboxCIDR    string
)

// agentCmd returns the `lattice sandbox agent` cobra command.
// This command is designed to run as PID 1 inside a gVisor runsc container.
// It is not intended for direct user invocation.
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
	cmd.Flags().StringVar(&agentSandboxIP, "sandbox-ip", "", "Static IP for gVisor virtual eth0 (e.g. 10.42.0.200)")
	cmd.Flags().StringVar(&agentSandboxGateway, "sandbox-gw", "", "Default gateway for gVisor virtual eth0")
	cmd.Flags().StringVar(&agentSandboxCIDR, "sandbox-cidr", "", "Subnet prefix length (e.g. 24)")
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

	// Configure gVisor's virtual eth0 with a static IP. In --network=sandbox
	// mode gVisor creates a virtual NIC but DHCP fails (K8s CNI has no DHCP
	// server). We configure eth0 manually before any network operations.
	if agentSandboxIP != "" && agentSandboxCIDR != "" {
		if err := configureEth0(agentSandboxIP, agentSandboxCIDR, agentSandboxGateway); err != nil {
			return fmt.Errorf("configure gVisor eth0: %w", err)
		}
	}

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

// configureEth0 assigns a static IP to the gVisor virtual eth0 and sets the
// default route. Used in --network=sandbox mode where DHCP is not available.
func configureEth0(ip, cidr, gateway string) error {
	// Diagnostics: list available interfaces.
	if out, err := exec.Command("ip", "link", "show").CombinedOutput(); err == nil {
		fmt.Printf("[sandbox-agent] interfaces:\n%s\n", out)
	}

	// ip addr add <ip>/<cidr> dev eth0
	addAddr := exec.Command("ip", "addr", "add", ip+"/"+cidr, "dev", "eth0")
	if out, err := addAddr.CombinedOutput(); err != nil {
		return fmt.Errorf("ip addr add %s/%s dev eth0: %w\n%s", ip, cidr, err, out)
	}

	// ip link set eth0 up
	linkUp := exec.Command("ip", "link", "set", "eth0", "up")
	if out, err := linkUp.CombinedOutput(); err != nil {
		return fmt.Errorf("ip link set eth0 up: %w\n%s", err, out)
	}

	// ip route add default via <gateway> dev eth0
	if gateway != "" {
		addRoute := exec.Command("ip", "route", "add", "default", "via", gateway, "dev", "eth0")
		if out, err := addRoute.CombinedOutput(); err != nil {
			return fmt.Errorf("ip route add default via %s: %w\n%s", gateway, err, out)
		}
	}
	return nil
}
