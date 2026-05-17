# Sandbox Agent 统一 NATS 注册 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 sandbox agent 的注册流程从 HTTP+NATS 双通道统一为纯 NATS，消灭注册竞态，并将 `lattice-agent-sandbox` 独立 binary 合并为 `lattice sandbox` 子命令。

**Architecture:** Server 端在 NATS `register` handler 中检测 `publicKey` 非空时走 enrollment token 路径；client 端新增 `RegisterSandboxViaNATS` 函数完成注册+等IP；`start.go` 删除 HTTP 注册逻辑；gVisor 代码通过 `//go:build pro` 条件编译进主 `lattice` binary。

**Tech Stack:** Go 1.25, NATS, wireguard-go, gVisor (lattice-shim), cobra, Ginkgo E2E

**Design Spec:** `docs/superpowers/specs/2026-05-17-sandbox-agent-unified-registration.md`

---

## 文件改动总览

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/server/server/server.go` | 修改 | Register handler 增加 sandbox 分支 |
| `internal/server/client/client.go` | 修改 | Register 带 publicKey 参数 |
| `internal/agent/node.go` | 修改 | messageHandler nil 竞态修复；NodeConfig 加 PublicKey/PrivateKey |
| `internal/agent/sandbox_register.go` | 新建 | RegisterSandboxViaNATS 函数 |
| `cmd/lattice/cmd/sandbox/sandbox.go` | 新建 | `lattice sandbox start` 子命令（公共部分） |
| `cmd/lattice/cmd/sandbox/sandbox_community.go` | 新建 | community stub (`//go:build !pro`) |
| `cmd/lattice/cmd/sandbox/sandbox_pro.go` | 新建 | pro 实现（从 lattice-agent-sandbox 迁移，`//go:build pro`） |
| `cmd/lattice/cmd/root.go` | 修改 | 注册 sandbox 子命令 |
| `cmd/lattice-agent-sandbox/` | 删除 | 整个目录 |
| `test/e2e/agent_sandbox_test.go` | 修改 | 更新 binary 名称引用 |

---

## Task 1: Server — Register handler 支持 sandbox enrollment token

**Files:**
- Modify: `internal/server/server/server.go:445-449`

背景：当前 `Register` 直接转发给 `peerController.Register`（K8s LatticeEnrollmentToken 路径）。需要在 `publicKey` 非空时改走 `agentRegService.RegisterAgent`（SQLite enrollment token 路径）。

- [ ] **Step 1.1: 修改 Register handler**

在 `internal/server/server/server.go` 中，将 `Register` 方法改为：

```go
func (s *Server) Register(content []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Sandbox path: enrollment token + client-side public key.
	// Detected by the presence of publicKey in the payload.
	// Regular agents never send publicKey during registration.
	if s.agentRegService != nil {
		var peek dto.PeerDto
		if jsonErr := json.Unmarshal(content, &peek); jsonErr == nil && peek.PublicKey != "" {
			return s.handleSandboxNATSRegister(ctx, peek)
		}
	}

	return s.peerController.Register(ctx, content)
}

// handleSandboxNATSRegister registers a sandbox agent via enrollment token over NATS.
// It delegates to agentRegistrationService.RegisterAgent and returns an infra.Peer
// with the agent JWT in the Token field (no PrivateKey — sandbox owns its own key).
func (s *Server) handleSandboxNATSRegister(ctx context.Context, peer dto.PeerDto) ([]byte, error) {
	result, err := s.agentRegService.RegisterAgent(ctx, service.AgentRegisterRequest{
		EnrollmentToken: peer.Token,
		AgentName:       peer.AppID,
		PublicKey:       peer.PublicKey,
		Sandbox:         v1alpha1.SandboxGVisor,
	})
	if err != nil {
		return nil, err
	}
	// Return infra.Peer with JWT in Token field.
	// PrivateKey is intentionally empty: sandbox generates its own key.
	returnPeer := &infra.Peer{
		Name:  peer.AppID,
		AppID: peer.AppID,
		Token: result.JWT,
	}
	data, err := json.Marshal(returnPeer)
	if err != nil {
		return nil, err
	}
	return data, nil
}
```

确认需要的 import 已存在（`service`, `v1alpha1`, `infra`, `dto`）。如需要，添加：

```go
"github.com/alatticeio/lattice/internal/agent/infra"
```

- [ ] **Step 1.2: 确认编译通过**

```bash
make build SERVICE=latticed
```

期望：编译成功，无错误。

- [ ] **Step 1.3: Commit**

```bash
git add internal/server/server/server.go
git commit -s -m "feat(agent-sandbox): NATS Register handler supports sandbox enrollment tokens"
```

---

## Task 2: Client — Register 携带 publicKey

**Files:**
- Modify: `internal/server/client/client.go:92-136`

当前 `Register` 不发送 `PublicKey`。需要在 payload 中带上，由 server 端判断是否走 sandbox 路径。

- [ ] **Step 2.1: 修改 Register 签名和 payload**

将 `client.go` 中的 `Register` 方法改为：

```go
// Register announces this node to the control plane. publicKey is optional:
// when non-empty (sandbox path), it is sent to the server so the server uses
// the client-generated key instead of generating one. The server then returns
// an infra.Peer with Token=agentJWT and empty PrivateKey.
func (c *Client) Register(ctx context.Context, token, interfaceName, publicKey string) (*infra.Peer, error) {
	if token == "" {
		return nil, fmt.Errorf("token is empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Error("get hostname failed", err)
		return nil, err
	}

	registryRequest := &dto.PeerDto{
		Name:                config.Conf.AppId,
		Hostname:            hostname,
		DisplayName:         config.Conf.Name,
		InterfaceName:       interfaceName,
		Platform:            runtime.GOOS,
		AppID:               config.Conf.AppId,
		PersistentKeepalive: 25,
		Port:                config.Conf.WgPort,
		Token:               token,
		PublicKey:           publicKey, // empty for regular agents
	}

	data, err := json.Marshal(registryRequest)
	if err != nil {
		return nil, err
	}

	data, err = c.RequestNats(ctx, "lattice.signals.peer", "register", data)
	if err != nil {
		return nil, fmt.Errorf("register failed. %v", err)
	}

	var node infra.Peer
	if err = json.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	return &node, nil
}
```

- [ ] **Step 2.2: 更新所有 Register 调用处**

`Register` 签名新增了 `publicKey string` 参数。找到所有调用处：

```bash
grep -rn "\.Register(ctx" internal/agent/ internal/server/ --include="*.go"
```

将 `node.ctrClient.Register(ctx, cfg.Token, node.Name)` 改为：
```go
node.ctrClient.Register(ctx, cfg.Token, node.Name, "")
```

（第四个参数空字符串 = 普通 agent 路径）

- [ ] **Step 2.3: 确认编译**

```bash
make build
```

- [ ] **Step 2.4: Commit**

```bash
git add internal/server/client/client.go internal/agent/node.go
git commit -s -m "feat(agent-sandbox): Register carries publicKey for sandbox enrollment path"
```

---

## Task 3: Node — 修复 messageHandler nil 竞态 + NodeConfig 扩展

**Files:**
- Modify: `internal/agent/node.go`

两个改动：
1. `messageHandler` 初始化提前到 NATS Subscribe 之前
2. `NodeConfig` 增加 `PublicKey`、`PrivateKey` 字段供 sandbox path 使用

- [ ] **Step 3.1: NodeConfig 加字段**

在 `NodeConfig` struct 中增加：

```go
// PublicKey, if non-empty, triggers the sandbox NATS registration path.
// The sandbox generates its own WireGuard key pair locally and sends only
// the public key to the server. The server creates LatticePeer + AgentIdentity
// and returns a JWT in infra.Peer.Token (no PrivateKey).
PublicKey string

// PrivateKey is the locally-generated WireGuard private key for sandbox mode.
// It is injected into node.current after registration since the server does
// not generate or store it.
PrivateKey string
```

- [ ] **Step 3.2: sandbox Register 分支 + PrivateKey 注入**

在 `NewNode` 的 Phase 2 中，将注册逻辑改为：

```go
if cfg.CurrentPeer != nil {
    node.current = cfg.CurrentPeer
} else if cfg.PublicKey != "" {
    // Sandbox NATS registration: send enrollment token + local public key.
    // Server returns infra.Peer with JWT in Token and empty PrivateKey.
    node.current, err = node.ctrClient.Register(ctx, cfg.Token, node.Name, cfg.PublicKey)
    if err != nil {
        return nil, err
    }
    // Inject locally-generated private key (server never sees it).
    node.current.PrivateKey = cfg.PrivateKey
} else {
    // Regular agent registration: server generates key pair.
    node.current, err = node.ctrClient.Register(ctx, cfg.Token, node.Name, "")
    if err != nil {
        return nil, err
    }
}
```

- [ ] **Step 3.3: 修复 messageHandler nil 竞态**

找到 Phase 3 中 `node.messageHandler = NewMessageHandler(...)` 这行，将其移动到 `natsSignalService.Subscribe(...)` 调用**之前**。

修改前（当前顺序）：
```go
// Phase 2:
natsSignalService.Subscribe(fmt.Sprintf(...), node.probeFactory.Handle)  // ← 订阅

// Phase 3:
node.messageHandler = NewMessageHandler(node, ...)  // ← messageHandler 才设置
```

修改后：
```go
// Phase 3 (提前):
node.messageHandler = NewMessageHandler(node, log.GetLogger("event-handler"), node.provisioner)

// Phase 2 (Subscribe 在 messageHandler 赋值之后):
natsSignalService.Subscribe(fmt.Sprintf(...), node.probeFactory.Handle)
```

注意：`NewMessageHandler` 需要 `node.provisioner`，而 `provisioner` 在 Phase 3 才创建。因此需要将整个 Phase 3 的 provisioner + messageHandler 初始化移动到 Subscribe 之前。

具体做法：将 Phase 3 的以下代码块整体上移到 `natsSignalService.Subscribe` 之前：

```go
node.bind = infra.NewBind(...)
node.iface = wg.NewDevice(iface, node.bind, ...)
if cfg.ProvisionerFactory != nil {
    node.provisioner = cfg.ProvisionerFactory(node.iface)
} else {
    // ... iptables/eBPF provisioner selection
}
node.messageHandler = NewMessageHandler(node, log.GetLogger("event-handler"), node.provisioner)
node.DeviceManager = wireguard.NewDeviceManager(...)
```

然后把 Subscribe 放在后面。

- [ ] **Step 3.4: 确认编译**

```bash
make build
make lint
```

- [ ] **Step 3.5: Commit**

```bash
git add internal/agent/node.go
git commit -s -m "fix(agent-sandbox): move messageHandler init before NATS subscribe to eliminate nil race"
```

---

## Task 4: 新建 RegisterSandboxViaNATS

**Files:**
- Create: `internal/agent/sandbox_register.go`

这个函数封装 sandbox 注册的两步流程（NATS register → 等待 IP），供 `cmd/lattice/cmd/sandbox/` 调用。

- [ ] **Step 4.1: 新建文件**

```go
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

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"io"
	"time"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/dto"
	managementnats "github.com/alatticeio/lattice/internal/server/nats"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// RegisterSandboxViaNATS performs the two-phase sandbox bootstrap over NATS:
//  1. NATS register with enrollment token + public key → receive agent JWT.
//  2. NATS GetNetMap poll until the controller assigns a VPN IP.
//
// Returns a fully-populated *infra.Peer with PrivateKey injected (never sent
// to the server). The returned peer is passed as NodeConfig.CurrentPeer to
// skip the NATS register inside NewNode.
//
// serverURL is the Lattice control-plane HTTP base URL (used only for
// NATS URL discovery). enrollmentToken is the one-time enrollment token.
func RegisterSandboxViaNATS(
	ctx context.Context,
	serverURL, enrollmentToken, agentName string,
	privKey wgtypes.Key,
) (*infra.Peer, error) {
	pubKey := privKey.PublicKey().String()

	natsURL, err := discoverNATSURL(ctx, serverURL)
	if err != nil {
		return nil, fmt.Errorf("discover NATS: %w", err)
	}

	natsClient, err := managementnats.NewNatsService(ctx, agentName, "sandbox", natsURL)
	if err != nil {
		return nil, fmt.Errorf("NATS connect: %w", err)
	}
	defer func() { _ = natsClient.Close() }()

	// Step 1: NATS register — sends enrollment token + public key.
	// Server creates LatticePeer + AgentIdentity and returns JWT in Token field.
	regPayload, _ := json.Marshal(&dto.PeerDto{
		AppID:     agentName,
		Token:     enrollmentToken,
		PublicKey: pubKey,
	})
	regData, err := natsClient.Request(ctx, "lattice.signals.peer", "register", regPayload)
	if err != nil {
		return nil, fmt.Errorf("NATS register: %w", err)
	}
	var registered infra.Peer
	if err = json.Unmarshal(regData, &registered); err != nil {
		return nil, fmt.Errorf("parse register response: %w", err)
	}
	agentJWT := registered.Token
	if agentJWT == "" {
		return nil, fmt.Errorf("server returned empty JWT")
	}

	// Step 2: Poll GetNetMap until the controller allocates a VPN IP.
	// The controller reconciles LatticePeer → IPAM → ConfigMap in a few seconds.
	getMapPayload, _ := json.Marshal(&dto.PeerDto{
		AppID:     agentName,
		PublicKey: pubKey,
		Token:     agentJWT,
	})
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		data, reqErr := natsClient.Request(ctx, "lattice.signals.peer", "GetNetMap", getMapPayload)
		if reqErr == nil {
			var msg infra.Message
			if json.Unmarshal(data, &msg) == nil && msg.Current != nil &&
				msg.Current.Address != nil && *msg.Current.Address != "" {
				peer := msg.Current
				peer.PrivateKey = privKey.String() // inject locally-generated key
				if peer.AppID == "" {
					peer.AppID = agentName
				}
				// Store JWT so node.Start() → GetNetworkMap can authenticate.
				peer.Token = agentJWT
				return peer, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("timed out waiting for VPN IP allocation")
}

// discoverNATSURLHTTP fetches the NATS URL from the server's discovery endpoint.
// This is the same helper as in node.go; extracted here to avoid circular deps
// if this file is moved to a separate package in the future.
func discoverNATSURLHTTP(serverURL string) (string, error) {
	resp, err := http.Get(serverURL + "/api/v1/discovery") //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("discovery request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			NatsURL string `json:"nats_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse discovery response: %w", err)
	}
	if result.Data.NatsURL == "" {
		return "", fmt.Errorf("empty NATS URL in discovery response")
	}
	return result.Data.NatsURL, nil
}
```

注意：`discoverNATSURL` 已存在于 `node.go`（使用 `context`）。在 `RegisterSandboxViaNATS` 中直接调用 `discoverNATSURL`（同 package，可直接复用）。删除上面文件里的 `discoverNATSURLHTTP`，改用 `discoverNATSURL(ctx, serverURL)`。

- [ ] **Step 4.2: 确认编译**

```bash
make build
```

- [ ] **Step 4.3: Commit**

```bash
git add internal/agent/sandbox_register.go
git commit -s -m "feat(agent-sandbox): RegisterSandboxViaNATS replaces HTTP+NATS two-step bootstrap"
```

---

## Task 5: 新建 lattice sandbox 子命令

**Files:**
- Create: `cmd/lattice/cmd/sandbox/sandbox.go`
- Create: `cmd/lattice/cmd/sandbox/sandbox_community.go`
- Create: `cmd/lattice/cmd/sandbox/sandbox_pro.go`
- Modify: `cmd/lattice/cmd/root.go`

将 `cmd/lattice-agent-sandbox/cmd/` 的逻辑迁移为 `lattice sandbox start` 子命令。

- [ ] **Step 5.1: 创建 sandbox.go（公共部分）**

`cmd/lattice/cmd/sandbox/sandbox.go`:

```go
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
	"github.com/spf13/cobra"
)

var (
	sandboxName         string
	sandboxServerURL    string
	sandboxToken        string
	sandboxProxyAddr    string
	sandboxForwardRules []string
	sandboxEgressAllow  string
	sandboxEgressDeny   bool
)

// SandboxCmd returns the top-level `sandbox` cobra command.
func SandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage sandboxed agent environments (Pro)",
	}
	cmd.AddCommand(startCmd())
	return cmd
}
```

- [ ] **Step 5.2: 创建 sandbox_community.go（stub）**

`cmd/lattice/cmd/sandbox/sandbox_community.go`:

```go
//go:build !pro

// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// ...

package sandbox

import (
	"errors"

	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a sandboxed agent environment (Pro only)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("lattice sandbox is a Lattice Pro feature")
		},
	}
	registerStartFlags(cmd)
	return cmd
}

func registerStartFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL")
	cmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token")
	cmd.Flags().StringVar(&sandboxProxyAddr, "proxy-addr", "", "HTTP forward proxy listen address")
	cmd.Flags().StringArrayVar(&sandboxForwardRules, "forward", nil, "Inbound forward rule: overlayPort:targetAddr")
	cmd.Flags().StringVar(&sandboxEgressAllow, "egress-allow", "", "Comma-separated allowed egress CIDRs")
	cmd.Flags().BoolVar(&sandboxEgressDeny, "egress-default-deny", false, "Whitelist egress mode")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
}
```

- [ ] **Step 5.3: 创建 sandbox_pro.go（从 lattice-agent-sandbox 迁移）**

`cmd/lattice/cmd/sandbox/sandbox_pro.go`:

将 `cmd/lattice-agent-sandbox/cmd/start.go` 的 `runStart` 函数迁移过来，核心改动：

1. 删除 `registerWithServer`、`fetchPeerViaNATS`、`discoverNATSURL` 函数
2. 使用 `agent.RegisterSandboxViaNATS()` 替代

```go
//go:build pro

// Copyright 2026 The Lattice Authors, Inc.
// Licensed under the Apache License, Version 2.0 (the "License").

package sandbox

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	shimfwd "github.com/alatticeio/lattice-shim/shim"
	"github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
	"github.com/spf13/cobra"
	wgdevice "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a sandboxed agent environment",
		Long: `Start creates a gVisor-based network sandbox attached to the Lattice overlay network.
It registers with the control plane via NATS, receives a VPN IP, and connects to peers
using the same ICE/LRP infrastructure as a regular agent. Policy is enforced by gVisor.`,
		RunE: runStart,
	}
	registerStartFlags(cmd)
	return cmd
}

func registerStartFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token (required)")
	cmd.Flags().StringVar(&sandboxProxyAddr, "proxy-addr", "", "HTTP forward proxy listen address (e.g. 127.0.0.1:1080)")
	cmd.Flags().StringArrayVar(&sandboxForwardRules, "forward", nil, "Inbound forward rule: overlayPort:targetAddr")
	cmd.Flags().StringVar(&sandboxEgressAllow, "egress-allow", "", "Comma-separated allowed egress CIDRs")
	cmd.Flags().BoolVar(&sandboxEgressDeny, "egress-default-deny", false, "Whitelist egress mode")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
}

func runStart(_ *cobra.Command, _ []string) error {
	// Generate WireGuard key pair locally. Private key never leaves this process.
	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("generate WireGuard key: %w", err)
	}

	// Build egress policy from flags.
	egressPolicy := shimfwd.EgressPolicy{DefaultDeny: sandboxEgressDeny}
	if sandboxEgressAllow != "" {
		for _, entry := range strings.Split(sandboxEgressAllow, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			_, cidr, cidrErr := net.ParseCIDR(entry)
			if cidrErr != nil {
				return fmt.Errorf("invalid egress CIDR %q: %w", entry, cidrErr)
			}
			egressPolicy.AllowedCIDRs = append(egressPolicy.AllowedCIDRs, *cidr)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register via NATS (replaces HTTP registerWithServer + fetchPeerViaNATS).
	// Blocks until the controller allocates a VPN IP.
	fmt.Printf("Registering sandbox %q via NATS...\n", sandboxName)
	agentconfig.Conf.AppId = sandboxName
	agentconfig.Conf.ServerUrl = sandboxServerURL

	currentPeer, err := agent.RegisterSandboxViaNATS(ctx, sandboxServerURL, sandboxToken, sandboxName, privKey)
	if err != nil {
		return fmt.Errorf("sandbox registration failed: %w", err)
	}
	localIP := ""
	if currentPeer.Address != nil {
		localIP = *currentPeer.Address
	}
	fmt.Printf("Registered %q, overlay IP=%s\n", sandboxName, localIP)

	// Create the gVisor sandbox and TUN adapter now that we know the VPN IP.
	policyChecker := shimfwd.NewEgressFilter(egressPolicy)
	sb, err := gvisor.New(gvisor.Config{
		ID:            sandboxName,
		LocalIP:       localIP,
		PolicyChecker: policyChecker,
	})
	if err != nil {
		return fmt.Errorf("create gVisor sandbox: %w", err)
	}
	defer sb.Close()

	tunDev := gvisor.NewTUNAdapter(sb.Channel(), gvisor.InjectIntoChannel(sb.Channel()))

	logger := agentlog.GetLogger("sandbox")

	nodeCfg := &agent.NodeConfig{
		Logger:      logger,
		Port:        51820,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomTUN:   tunDev,
		CustomName:  sandboxName,
		CurrentPeer: currentPeer, // skip NATS register; already done above
		ProvisionerFactory: func(dev *wgdevice.Device) provision.Provisioner {
			return gvisor.NewSandboxProvisionerFactory(localIP, sandboxName)(dev)
		},
	}

	node, err := agent.NewNode(ctx, nodeCfg)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}

	agentJWT := currentPeer.Token
	node.GetNetworkMap = func() (*agent.infra.Message, error) {
		msg, err := node.GetNetMap(agentJWT)
		if err != nil {
			logger.Error("get network map failed", err)
			return nil, err
		}
		return msg, nil
	}

	if sandboxProxyAddr != "" {
		if err := sb.StartProxy(sandboxProxyAddr); err != nil {
			return fmt.Errorf("start proxy: %w", err)
		}
		fmt.Printf("HTTP proxy listening on %s\n", sandboxProxyAddr)
	}

	var fwdRules []shimfwd.ForwardRule
	for _, r := range sandboxForwardRules {
		rule, parseErr := parseForwardRule(r)
		if parseErr != nil {
			return fmt.Errorf("parse --forward %q: %w", r, parseErr)
		}
		fwdRules = append(fwdRules, rule)
	}
	if len(fwdRules) > 0 {
		fl := shimfwd.NewForwardListener(sb.Netstack(), sb.LocalIP(), fwdRules)
		if err := fl.Start(ctx); err != nil {
			return fmt.Errorf("start forward listener: %w", err)
		}
	}

	if err := node.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}

	// Periodic refresh as NATS push fallback (15s).
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if refreshErr := node.RefreshConfig(ctx); refreshErr != nil {
					logger.Warn("periodic config refresh failed", "err", refreshErr)
				}
			}
		}
	}()

	fmt.Printf("Sandbox %q ready, overlay IP=%s\n", sandboxName, localIP)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()
	fmt.Println("\nShutting down...")
	_ = node.Stop()
	return nil
}

func parseForwardRule(s string) (shimfwd.ForwardRule, error) {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return shimfwd.ForwardRule{}, fmt.Errorf("expected overlayPort:targetAddr, got %q", s)
	}
	portStr := s[:idx]
	target := s[idx+1:]
	if target == "" {
		return shimfwd.ForwardRule{}, fmt.Errorf("empty targetAddr in %q", s)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil || port < 1 || port > 65535 {
		return shimfwd.ForwardRule{}, fmt.Errorf("invalid overlay port %q", portStr)
	}
	return shimfwd.ForwardRule{
		OverlayPort: uint16(port),
		TargetAddr:  target,
	}, nil
}
```

注意：`node.GetNetworkMap` 赋值中 `agent.infra.Message` 需要改为 `infra.Message`，加对应 import。

- [ ] **Step 5.4: 注册到 root.go**

在 `cmd/lattice/cmd/root.go` 中，找到 `rootCmd.AddCommand(...)` 部分，加入：

```go
import "github.com/alatticeio/lattice/cmd/lattice/cmd/sandbox"

// 在 init 或 Execute 中:
rootCmd.AddCommand(sandbox.SandboxCmd())
```

- [ ] **Step 5.5: 确认编译**

```bash
make build                    # community build
make build EDITION=pro        # pro build
```

- [ ] **Step 5.6: Commit**

```bash
git add cmd/lattice/cmd/sandbox/ cmd/lattice/cmd/root.go
git commit -s -m "feat(agent-sandbox): add 'lattice sandbox start' subcommand (Pro)"
```

---

## Task 6: 删除 cmd/lattice-agent-sandbox

**Files:**
- Delete: `cmd/lattice-agent-sandbox/` (entire directory)
- Modify: `test/e2e/agent_sandbox_test.go` (binary 引用)
- Modify: `Makefile` (build target)

- [ ] **Step 6.1: 确认 E2E 测试里的 binary 引用**

```bash
grep -rn "lattice-agent-sandbox" test/ Makefile --include="*.go" --include="Makefile"
```

将所有 `lattice-agent-sandbox` 引用改为 `lattice sandbox`（注意 E2E 里可能是通过 exec 调用的命令）。

- [ ] **Step 6.2: 删除旧目录**

```bash
rm -rf cmd/lattice-agent-sandbox/
```

- [ ] **Step 6.3: 更新 Makefile**

找到构建 `lattice-agent-sandbox` 的 target，删除或替换为说明（sandbox 功能已集成进 `lattice`）。

- [ ] **Step 6.4: 确认完整构建**

```bash
make build
make build EDITION=pro
make lint
```

- [ ] **Step 6.5: Commit**

```bash
git add -A
git commit -s -m "refactor(agent-sandbox): remove standalone binary, sandbox is now 'lattice sandbox start'"
```

---

## Task 7: tunAdapter busy-loop 修复（可选但推荐）

**Files:**
- Modify: `internal/agent/gvisor/wg_device.go:54-62`

将非阻塞的 `t.ch.Read()` 改为阻塞等待，避免 wireguard-go TUN 读 goroutine 以 100% CPU 自旋。

- [ ] **Step 7.1: 修改 tunAdapter.Read**

```go
func (t *tunAdapter) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	if t.closed.Load() {
		return 0, errors.New("tun: closed")
	}

	// Use ReadContext with a short deadline so wireguard-go's goroutine can
	// check t.closed periodically instead of spinning on nil returns.
	// gVisor channel.Endpoint does not have a blocking Read, so we poll
	// with a short sleep to avoid a hot loop.
	for {
		pkt := t.ch.Read()
		if pkt != nil {
			defer pkt.DecRef()
			data := pkt.ToView()
			if data == nil || data.Size() == 0 {
				continue
			}
			n := data.Size()
			if n+offset > len(bufs[0]) {
				n = len(bufs[0]) - offset
			}
			copy(bufs[0][offset:], data.AsSlice()[:n])
			sizes[0] = n
			return 1, nil
		}
		if t.closed.Load() {
			return 0, errors.New("tun: closed")
		}
		// Brief yield to avoid 100% CPU spin when channel is empty.
		// This goroutine is wireguard-go's TUN reader; a 1ms pause is acceptable.
		time.Sleep(time.Millisecond)
	}
}
```

需要在 import 中加 `"time"`。

- [ ] **Step 7.2: 确认编译**

```bash
make build EDITION=pro
```

- [ ] **Step 7.3: Commit**

```bash
git add internal/agent/gvisor/wg_device.go
git commit -s -m "fix(agent-sandbox): replace tunAdapter.Read busy-loop with 1ms yield"
```

---

## Task 8: E2E 验证

- [ ] **Step 8.1: 确认 E2E 测试用新命令**

检查 `test/e2e/agent_sandbox_test.go`，确认沙盒容器镜像使用的是 `lattice sandbox start` 而非 `lattice-agent-sandbox start`。

- [ ] **Step 8.2: 本地跑 E2E（如有集群）**

```bash
make test-e2e
```

期望：`Agent Sandbox` 系列测试全部通过，不再出现 "no known endpoint for peer" 超时。

- [ ] **Step 8.3: 最终 lint 和构建检查**

```bash
make lint
make build
make build EDITION=pro
```

---

## 自检清单

- [ ] Server `Register` handler: sandbox 路径仅当 `publicKey != ""` 时触发，不影响普通 agent
- [ ] `RegisterSandboxViaNATS` 不泄漏私钥（仅在本地使用，不序列化发送）
- [ ] `messageHandler` 在 Subscribe 之前初始化（竞态修复）
- [ ] `lattice sandbox start` community build 返回 402 错误
- [ ] `lattice sandbox start` pro build 正常启动
- [ ] `cmd/lattice-agent-sandbox/` 已完全删除
- [ ] Makefile 没有残留 `lattice-agent-sandbox` target
- [ ] E2E 测试通过
