# VitePress Docs Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于最新 Agent Platform 实现重构 VitePress 文档站点，新增 `/agent/` 独立路径、修复 `/design/` 死链、更新主页与核心文档页面内容。

**Architecture:** 纯文档变更，无 Go/Vue 逻辑改动（仅 LatticeSandbox.vue 演示文字）。新建 9 个 markdown 文件，修改 6 个文件，所有变更均在 `docs/` 目录内完成。构建验证通过 `pnpm docs:build` 检查死链。

**Tech Stack:** VitePress 1.6、Markdown、TypeScript（config.mts）、Vue 3（LatticeSandbox.vue）

---

### Task 1: 更新主页 index.md

**Files:**
- Modify: `docs/index.md`

- [ ] **Step 1: 写入新的 index.md**

用以下内容完整替换 `docs/index.md`：

```markdown
---
layout: home

hero:
  name: "Lattice"
  text: "WireGuard Overlay Network for AI Workloads"
  tagline: Zero-privilege agent sandbox · Kubernetes-native · Open-core
  image:
    src: /logo.svg
    alt: Lattice
  actions:
    - theme: brand
      text: Quick Start →
      link: /guide/quickstart
    - theme: alt
      text: Agent Platform
      link: /agent/
    - theme: alt
      text: GitHub
      link: https://github.com/alatticeio/lattice

features:
  - icon: 🤖
    title: Agent Sandbox
    details: gVisor zero-privilege sandbox for AI agents — NATS registration, ICE/LRP tunneling, credential persistence. Community Edition.
  - icon: 🔐
    title: WireGuard Mesh
    details: Automatic NAT traversal via ICE/STUN/TURN with built-in LRP relay over QUIC
  - icon: ☸️
    title: K8s Native
    details: CRD operator (kubebuilder) for Kubernetes clusters, CLI agent for devices — same network plane
  - icon: 🛡️
    title: Network Policies
    details: Default-deny, label-based ACLs with eBPF (Pro) or iptables enforcement
  - icon: 🧠
    title: MCP + ChatOps
    details: Natural language network management via MCP Server — works with Claude, Cursor, and any MCP client
  - icon: 🏷️
    title: Open-Core
    details: Community edition is free forever. Pro adds eBPF, NATS audit, compliance automation, and AI debugging
---

## Feature Map

| Module | Community | Pro |
|--------|-----------|-----|
| WireGuard Tunnels | ✅ | ✅ |
| NAT Traversal (ICE/STUN/TURN) | ✅ | ✅ |
| LRP Relay (QUIC) | ✅ | ✅ |
| K8s CRD Operator | ✅ | ✅ |
| Dashboard UI | ✅ | ✅ |
| Agent Sandbox (gVisor) | ✅ | ✅ + EgressFilter |
| Sub-agent Delegate API | ✅ | ✅ |
| MCP Tool Tracing | ✅ | ✅ |
| Label-based ACLs | ✅ (iptables) | ✅ (eBPF) |
| Cluster Peering | ✅ | ✅ |
| Multi-Tenant Workspaces | ✅ | ✅ |
| MCP Server & ChatOps | ✅ | ✅ |
| Network Topology Map | ✅ | ✅ |
| NATS Flow Audit | — | ✅ |
| Policy Engine | Basic | Advanced |
| Time-Travel Debugging | — | ✅ |
| Compliance Reports | — | ✅ |
| Audit Logging | Basic | Advanced |

## Ready to Try?

[Quick Start](/guide/quickstart) · [Agent Platform](/agent/) · [GitHub](https://github.com/alatticeio/lattice)
```

- [ ] **Step 2: 验证文件内容**

```bash
head -20 /Users/francis/workspc/lattice/docs/index.md
```

Expected: 看到 `text: "WireGuard Overlay Network for AI Workloads"`

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/index.md
git commit -s -m "docs(site): update home page to highlight Agent Platform"
```

---

### Task 2: 重构 config.mts（导航栏 + 侧边栏）

**Files:**
- Modify: `docs/.vitepress/config.mts`

- [ ] **Step 1: 写入新的 config.mts**

用以下内容完整替换 `docs/.vitepress/config.mts`：

```typescript
import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Lattice',
  description: 'WireGuard overlay network for AI workloads and infrastructure',
  cleanUrls: true,
  vite: {
    ssr: {
      noExternal: ['@xterm/xterm', '@xterm/addon-fit', '@xterm/addon-web-links'],
    },
  },
  base: '/',
  ignoreDeadLinks: [
    /^http:\/\/localhost/,
    /^\/demo(?:\/|$)/,
    /^\/features\//,
  ],
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Lattice Docs',
    nav: [
      { text: 'Docs', link: '/guide/quickstart' },
      { text: 'Deploy', link: '/deploy/all-in-one' },
      { text: 'Agent', link: '/agent/' },
      { text: 'AI', link: '/ai/' },
      { text: 'Blog', link: '/blog/' },
      { text: 'Compare', link: '/comparison' },
    ],
    sidebar: {
      // ── User-facing docs ──────────────────────────────────────────────────
      '/guide/': userSidebar(),
      '/deploy/': userSidebar(),
      '/config/': userSidebar(),
      '/features/': userSidebar(),
      '/faq/': userSidebar(),

      // ── Agent Platform ────────────────────────────────────────────────────
      '/agent/': agentSidebar(),

      // ── AI capabilities ───────────────────────────────────────────────────
      '/ai/': aiSidebar(),

      // ── Internal / developer docs ─────────────────────────────────────────
      '/design/': designSidebar(),
      '/adr/': designSidebar(),
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/alatticeio/lattice' },
    ],
    footer: {
      message: 'Built with Lattice · <a href="https://alattice.io">Console</a>',
      copyright: '© 2026 The Lattice Authors',
    },
  },
})

function userSidebar() {
  return [
    {
      text: 'Getting Started',
      items: [
        { text: 'Quick Start', link: '/guide/quickstart' },
        { text: 'Installation', link: '/guide/installation' },
        { text: 'Agent Setup', link: '/guide/agent' },
      ],
    },
    {
      text: 'Deployment',
      items: [
        { text: 'All-in-One', link: '/deploy/all-in-one' },
        { text: 'Helm Chart', link: '/deploy/helm' },
        { text: 'K8s Operator', link: '/deploy/k8s-operator' },
        { text: 'Configuration', link: '/config/reference' },
      ],
    },
    {
      text: 'Features',
      items: [
        {
          text: 'Networking',
          collapsed: false,
          items: [
            { text: 'Nodes & Peers', link: '/features/nodes' },
            { text: 'Network Policies', link: '/features/policies' },
            { text: 'Topology Viewer', link: '/features/topology' },
            { text: 'Cluster Peering', link: '/features/cluster-peering' },
            { text: 'Network Peering', link: '/features/network-peering' },
          ],
        },
        {
          text: 'Platform',
          collapsed: false,
          items: [
            { text: 'Workspaces & Members', link: '/features/workspaces' },
            { text: 'Relays', link: '/features/relays' },
            { text: 'Monitoring', link: '/features/monitoring' },
            { text: 'Alerts & Rules', link: '/features/alerts' },
            { text: 'Audit Logging', link: '/features/audit' },
            { text: 'Notifications', link: '/features/notifications' },
            { text: 'Approvals', link: '/features/approvals' },
          ],
        },
        {
          text: 'Account',
          collapsed: true,
          items: [
            { text: 'Profile & Settings', link: '/features/account' },
            { text: 'Billing', link: '/features/billing' },
          ],
        },
      ],
    },
    {
      text: 'FAQ',
      items: [
        { text: 'eBPF & Agent Sandbox', link: '/faq/ebpf-sandbox' },
      ],
    },
    {
      text: 'How-to Guides',
      items: [
        { text: 'Multi-Cloud Peering', link: '/guide/multi-cloud-peering' },
        { text: 'Remote Device Onboarding', link: '/guide/remote-device-onboarding' },
        { text: 'AI Agent Zero-Trust', link: '/guide/ai-agent-zero-trust' },
      ],
    },
  ]
}

function agentSidebar() {
  return [
    {
      text: 'Agent Platform',
      items: [
        { text: 'Overview', link: '/agent/' },
        { text: 'Sandbox (Community)', link: '/agent/sandbox' },
        { text: 'Sandbox (Pro)', link: '/agent/sandbox-pro' },
        { text: 'Sub-agent Delegate API', link: '/agent/delegate-api' },
      ],
    },
  ]
}

function aiSidebar() {
  return [
    {
      text: 'AI Capabilities',
      items: [
        { text: 'Overview', link: '/ai/' },
        { text: 'MCP Server & ChatOps', link: '/ai/mcp-server' },
        { text: 'Agent Enrollment API', link: '/ai/agent-enrollment' },
        { text: 'Intent Engine (Pro)', link: '/ai/intent-engine' },
        { text: 'Time-Travel Debugging (Pro)', link: '/ai/debugging' },
        { text: 'Compliance (Pro)', link: '/ai/compliance' },
      ],
    },
  ]
}

function designSidebar() {
  return [
    {
      text: 'Architecture',
      items: [
        { text: 'Overview', link: '/design/architecture' },
        { text: 'Sandbox Architecture', link: '/design/sandbox' },
        { text: 'ICE Connection', link: '/design/ice-connection' },
        { text: 'ICE + WireGuard Mux', link: '/design/ice-wireguard-mux' },
        { text: 'WRRP / QUIC', link: '/design/wrrp-quic' },
      ],
    },
    {
      text: 'ADR',
      items: [
        { text: '0001 - Performance Benchmark', link: '/adr/0001-performance-benchmark-design' },
      ],
    },
  ]
}
```

- [ ] **Step 2: 验证 TypeScript 语法**

```bash
cd /Users/francis/workspc/lattice/docs && npx tsc --noEmit --strict docs/.vitepress/config.mts 2>&1 || echo "tsc check done"
```

Expected: 无 type error（或只有 noEmit 相关提示）

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/.vitepress/config.mts
git commit -s -m "docs(config): add agent nav, split ai/agent sidebars, fix design sidebar"
```

---

### Task 3: 新建 agent/index.md — Agent Platform 概述

**Files:**
- Create: `docs/agent/index.md`

- [ ] **Step 1: 创建文件**

```bash
mkdir -p /Users/francis/workspc/lattice/docs/agent
```

写入 `docs/agent/index.md`：

```markdown
---
title: Agent Platform
---

# Agent Platform

Lattice provides two ways for AI agents to join the WireGuard mesh:

| Method | Command | Isolation | Privilege |
|--------|---------|-----------|-----------|
| **Regular node** | `lattice up` | None (host process) | root / `CAP_NET_ADMIN` |
| **Sandbox** | `lattice sandbox start` | gVisor user-space netstack | **Zero-privilege** |

The Sandbox is the recommended approach for AI agent workloads — it runs entirely in user space, requires no kernel capabilities, and integrates with the same NATS signaling, ICE, and LRP infrastructure as regular nodes.

## Community vs Pro

| Capability | Community | Pro |
|-----------|-----------|-----|
| gVisor user-space network stack | ✅ | ✅ |
| NATS registration + ICE/LRP tunneling | ✅ | ✅ |
| Credential persistence (restart-safe) | ✅ | ✅ |
| Local file audit (`/tmp/lattice-audit-<name>.jsonl`) | ✅ | ✅ |
| Egress policy filtering (`EgressFilter`, `--egress-allow`) | ❌ | ✅ |
| Inbound port forwarding (`--forward`) | ❌ | ✅ |
| HTTP forward proxy (`--proxy-addr`) | ❌ | ✅ |
| NATS flow audit (server-side `la_flow_events`) | ❌ | ✅ |

## Sub-agent Architecture

Agents can delegate identity to child agents via the **Delegate API** — a parent agent issues a short-TTL token that a sub-agent uses to self-register with a constrained identity (`AgentIdentity.spec.parentRef`).

## Quick Navigation

- [Sandbox (Community)](/agent/sandbox) — full guide: startup flow, CLI reference, credential persistence, AI framework integration
- [Sandbox (Pro)](/agent/sandbox-pro) — EgressFilter, ForwardListener, HTTP proxy, NATS audit
- [Sub-agent Delegate API](/agent/delegate-api) — CRD fields, HTTP endpoint, curl and Python examples
```

- [ ] **Step 2: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/agent/index.md
git commit -s -m "docs(agent): add Agent Platform overview page"
```

---

### Task 4: 新建 agent/sandbox.md — Community Sandbox 完整指南

**Files:**
- Create: `docs/agent/sandbox.md`

- [ ] **Step 1: 创建文件**

写入 `docs/agent/sandbox.md`：

```markdown
---
title: Agent Sandbox (Community)
---

# Agent Sandbox — Community Edition

`lattice sandbox start` is a zero-privilege sandbox command built into the main `lattice` CLI. It fuses the **gVisor user-space network stack** with the **Lattice WireGuard overlay**, letting AI agent processes run as a regular user while getting a full Lattice network identity.

## Network Architecture

```
                ┌─────────────────────────────┐
                │       gVisor Sandbox         │
                │                              │
  Agent process ──▶  gVisor netstack (tcpip)   │
  connect()         │                          │
                │   TUNAdapter (channel bridge)│
                │   │                          │
                │   wireguard-go Device        │
                └──────────┬───────────────────┘
                           │ UDP :51820
                ┌──────────▼───────────────────┐
                │   FilteringUDPMux             │
                │   STUN ──▶ ICE agent          │
                │   non-STUN ──▶ WG DefaultBind │
                └──────────┬───────────────────┘
                           │
          ┌────────────────┴──────────────┐
          │  ICE succeeds                 │  ICE fails
          ▼                               ▼
    Direct P2P                    LRP relay (QUIC/TCP)
```

The sandbox uses the **same signaling path** as a regular node: `NATS → ProbeFactory → ICE/LRP`. gVisor only replaces the kernel TUN device; upper-layer logic is unaware.

## Startup Flow

1. **Load credentials** from `/etc/lattice/sandbox-credentials.json` (container restart recovery)
   - Found → `ResumeSandboxViaNATS(jwt, privKey)` → skip registration, retrieve VPN IP
   - Not found → new registration
2. **New registration**: generate WireGuard key → `RegisterSandboxViaNATS(serverURL, token, name, privKey)` → save credentials (mode `0600`)
3. If `peer.LrpUrl != ""` → start LRP relay client
4. Start `fileAuditWriter` → `/tmp/lattice-audit-<name>.jsonl`
5. `gvisor.New(Config{ID, LocalIP, AuditWriter, PolicyChecker: nil})` — Community passes no PolicyChecker, all egress allowed
6. `gvisor.NewTUNAdapter(sb.Channel(), InjectIntoChannel)`
7. `agent.NewNode(ctx, NodeConfig{CustomTUN, CurrentPeer, ...})` — shares NATS + ICE + LRP infrastructure
8. `node.Start(ctx)` → heartbeat every 30s, config refresh every 15s

## Quick Start

### Prerequisites

- `latticed` running (control plane)
- An enrollment token (create via dashboard or `lattice token create`)

### Start a Sandbox

```bash
lattice sandbox start \
  --name my-agent \
  --server-url http://localhost:8080 \
  --token lt-xxxxxxxx
```

Expected output:

```
INF Loading credentials path=/etc/lattice/sandbox-credentials.json
INF Registering sandbox via NATS name=my-agent
INF Sandbox credentials saved
INF gVisor sandbox initialized id=my-agent localIP=10.42.0.5
INF TUN adapter started
INF Node started, heartbeat every 30s
```

### Verify Connectivity

From another node in the same workspace:

```bash
ping 10.42.0.5
```

## CLI Reference

```
lattice sandbox start [flags]

Flags:
  --name         string   Sandbox identity name (required)
  --server-url   string   LatticeD URL (default: http://localhost:8080)
  --token        string   Enrollment token (required)
```

## Credential Persistence

Credentials are saved to `/etc/lattice/sandbox-credentials.json` with mode `0600` on first registration. On restart, the sandbox resumes the existing identity via NATS without re-registering — the agent's overlay IP and WireGuard key remain stable.

To force a fresh registration, delete the credentials file:

```bash
rm /etc/lattice/sandbox-credentials.json
```

## Audit Log

All network activity is written to `/tmp/lattice-audit-<name>.jsonl` as JSON lines:

```json
{"timestamp":"2026-05-18T10:00:01Z","srcIP":"10.42.0.5","dstIP":"10.42.0.1","proto":"tcp","dstPort":443,"action":"allow"}
```

## AI Framework Integration

### Python / LangGraph

```python
import subprocess
import asyncio

async def run_with_sandbox():
    proc = subprocess.Popen([
        "lattice", "sandbox", "start",
        "--name", "langgraph-agent",
        "--server-url", "http://lattice.internal:8080",
        "--token", "lt-workspace-token",
    ])
    try:
        # Your LangGraph agent code runs here with Lattice network identity
        await your_langgraph_workflow()
    finally:
        proc.terminate()
```

### Kubernetes Init Container

```yaml
initContainers:
  - name: lattice-sandbox
    image: ghcr.io/alatticeio/lattice:latest
    command: ["lattice", "sandbox", "start"]
    args:
      - --name=$(POD_NAME)
      - --server-url=http://latticed.lattice-system:8080
      - --token=$(LATTICE_TOKEN)
    env:
      - name: POD_NAME
        valueFrom:
          fieldRef:
            fieldPath: metadata.name
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
```

| Framework | Integration Point |
|-----------|------------------|
| LangGraph | Process wrapper in `StateGraph` lifespan |
| AutoGen | `ConversableAgent` init/del hooks |
| Claude Agent SDK | Agent startup script |
| Kubernetes Job | Init container + sidecar |
```

- [ ] **Step 2: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/agent/sandbox.md
git commit -s -m "docs(agent): add Community sandbox guide with startup flow and CLI reference"
```

---

### Task 5: 新建 agent/sandbox-pro.md 和 agent/delegate-api.md

**Files:**
- Create: `docs/agent/sandbox-pro.md`
- Create: `docs/agent/delegate-api.md`

- [ ] **Step 1: 创建 sandbox-pro.md**

写入 `docs/agent/sandbox-pro.md`：

```markdown
---
title: Agent Sandbox (Pro)
---

# Agent Sandbox — Pro Edition

The Pro sandbox extends the [Community sandbox](/agent/sandbox) with egress filtering, inbound port forwarding, an HTTP proxy, and server-side NATS flow auditing.

## Additional Flags

```
lattice sandbox start [flags]

Pro-only flags:
  --egress-allow   strings   CIDR ranges to allow outbound (default: deny-all)
  --forward        strings   Overlay port → host address mappings (e.g. 8080:127.0.0.1:8080)
  --proxy-addr     string    Local address for HTTP forward proxy (e.g. 127.0.0.1:3128)
```

## EgressFilter

EgressFilter implements `PolicyChecker` — it intercepts every outbound connection attempt from the gVisor netstack and evaluates it against a CIDR allowlist.

```bash
# Only allow outbound to the tool service at 10.42.0.10 and HTTPS
lattice sandbox start \
  --name agent-001 \
  --server-url http://lattice.internal:8080 \
  --token lt-xxx \
  --egress-allow 10.42.0.10/32 \
  --egress-allow 0.0.0.0/0:443
```

If no `--egress-allow` flags are provided, all egress is denied. Denied connections are logged to the audit file.

## ForwardListener

Forward inbound connections from the overlay network to a host-local address:

```bash
# Accept connections on overlay port 8080, forward to localhost:8080
lattice sandbox start \
  --name api-agent \
  --server-url http://lattice.internal:8080 \
  --token lt-xxx \
  --forward 8080:127.0.0.1:8080
```

Multiple `--forward` flags are supported.

## HTTP Forward Proxy

Expose an HTTP CONNECT proxy inside the gVisor sandbox. Agent processes using this proxy inherit Lattice network identity for all outbound HTTP/HTTPS traffic.

```bash
lattice sandbox start \
  --name browser-agent \
  --server-url http://lattice.internal:8080 \
  --token lt-xxx \
  --proxy-addr 127.0.0.1:3128
```

In your agent process:

```bash
export https_proxy=http://127.0.0.1:3128
curl https://internal-api.example.com/data
```

## NATS Flow Audit (Server-side)

Pro adds server-side persistence of network flow events. The sandbox publishes to `lattice.audit.flow` on NATS; the control plane's `AuditConsumer` persists events to the `la_flow_events` database table.

> **Status:** Server-side pipeline is complete (`AuditConsumer` + `la_flow_events`). The sandbox-side `natsAuditWriter` is in development — sandbox currently writes to local file only.

Query audit events via the dashboard or API:

```bash
GET /api/v1/audit/flow?agentName=agent-001&from=2026-05-18T00:00:00Z
```
```

- [ ] **Step 2: 创建 delegate-api.md**

写入 `docs/agent/delegate-api.md`：

```markdown
---
title: Sub-agent Delegate API
---

# Sub-agent Delegate API

The Delegate API lets a parent agent issue a short-TTL enrollment token to a sub-agent. The sub-agent self-registers with a constrained identity that references its parent via `AgentIdentity.spec.parentRef`.

## Use Case

A coordinator agent spins up multiple task-specific sub-agents dynamically. Each sub-agent gets a time-bound token scoped to a specific policy preset — if it's compromised, the blast radius is limited to its TTL and policy scope.

## CRD Field

```yaml
# api/v1alpha1/AgentIdentity
spec:
  parentRef:
    name: coordinator-agent-001      # parent AgentIdentity name
    namespace: default
  ttl: 30m
  policyPreset: sandboxed
```

## HTTP Endpoint

```http
POST /api/v1/agents/:id/delegate
Authorization: Bearer <parent-agent-token>
Content-Type: application/json
```

**Request:**

```json
{
  "subAgentName": "task-executor-42",
  "ttl": "30m",
  "policyPreset": "sandboxed"
}
```

**Response:**

```json
{
  "enrollmentToken": "lt-delegate-xxxxxxxx",
  "expiresAt": "2026-05-18T10:30:00Z",
  "parentRef": {
    "name": "coordinator-agent-001",
    "namespace": "default"
  }
}
```

## Examples

### curl

```bash
# Parent agent requests a delegate token for a sub-agent
curl -s http://lattice.internal:8080/api/v1/agents/peer-coordinator-001/delegate \
  -X POST \
  -H "Authorization: Bearer $PARENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "subAgentName": "task-executor-42",
    "ttl": "30m",
    "policyPreset": "sandboxed"
  }'
```

### Python SDK

```python
from lattice_sdk import LatticeAgent

async with LatticeAgent(
    server="http://lattice.internal:8080",
    token="lt-coordinator-token",
    agent_name="coordinator",
    policy_preset="coordinator",
) as coordinator:
    # Delegate a token to a sub-agent
    delegate = await coordinator.delegate(
        sub_agent_name="task-executor-42",
        ttl="30m",
        policy_preset="sandboxed",
    )

    # Pass the token to the sub-agent process
    await spawn_sub_agent(token=delegate.enrollment_token)
```

## Token Lifecycle

1. Parent agent calls `POST /api/v1/agents/:id/delegate`
2. Server calls `DelegateToken()` in `service/agent_registration.go`
3. Returns a short-TTL token with `parentRef` set
4. Sub-agent uses token in `lattice sandbox start --token <delegate-token>`
5. Manager reconciler deletes the sub-agent's `LatticePeer` when TTL expires
```

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/agent/sandbox-pro.md docs/agent/delegate-api.md
git commit -s -m "docs(agent): add Pro sandbox and Sub-agent Delegate API pages"
```

---

### Task 6: 更新 ai/index.md 和 ai/agent-enrollment.md

**Files:**
- Modify: `docs/ai/index.md`
- Modify: `docs/ai/agent-enrollment.md`

- [ ] **Step 1: 更新 ai/index.md 的 Layer 2 描述**

用以下内容完整替换 `docs/ai/index.md`：

```markdown
---
title: AI Capabilities Overview
---

# AI Capabilities Overview

Lattice provides a **five-layer AI capability model** spanning developer productivity to enterprise compliance. All AI features are accessible through the built-in dashboard, the MCP protocol (Claude Desktop / Claude Code / Cursor), and REST APIs.

```
+-----------------------------------------------------------------+
|  Layer 5: Compliance-as-Conversation                            |
|  SOC2/PCI-DSS/HIPAA compliance report generation                |
+-----------------------------------------------------------------+
|  Layer 4: Time-Travel Network Debugging                         |
|  Network state snapshots + AI root cause analysis               |
+-----------------------------------------------------------------+
|  Layer 3: Network Intent Engine (Pro)                           |
|  Natural language intent → CRD change plans                     |
+-----------------------------------------------------------------+
|  Layer 2: Zero-Trust for AI Agent Fleets                        |
|  Agent identity, TTL, network isolation — two methods:          |
|    • HTTP API  (agent-enroll) → policy preset + SDK             |
|    • CLI sandbox (lattice sandbox start) → gVisor zero-priv     |
+-----------------------------------------------------------------+
|  Layer 1: MCP Server + AI ChatOps Write Tools                   |
|  Entry point and execution engine for all upper layers          |
+-----------------------------------------------------------------+
```

## Layer 1 — MCP Server + ChatOps ([Details](./mcp-server))

The foundation layer. Exposes Lattice's full network management capability via the **Model Context Protocol (MCP)**.

- Read tools: list peers, policies, networks, check connectivity
- Write tools: create/delete peers, manage policies (with human-in-the-loop approval)
- Works with Claude Desktop, Claude Code, Cursor, and any MCP client
- All write operations go through `WorkflowService` for approval by default
- MCP tool spans are traced and persisted to `la_tool_spans` table

## Layer 2 — Zero-Trust AI Agent Networking ([Details](./agent-enrollment))

AI agents get their own WireGuard identity with time-bound enrollment and network isolation. Two access methods:

**HTTP Enrollment API** — agents self-register via `POST /api/v1/agent-enroll`:
- Policy presets: `sandboxed`, `coordinator`, `isolated`
- TTL-based auto-destruction via Manager reconciler
- Python SDK (`lattice-sdk-python`) for LangGraph, AutoGen, Claude Agent SDK

**CLI Sandbox** — `lattice sandbox start` provides a gVisor zero-privilege sandbox:
- No root or `CAP_NET_ADMIN` required
- Full Lattice network identity via NATS + ICE/LRP
- See [Agent Platform docs](/agent/) for complete reference

## Layer 3 — Network Intent Engine (Pro) ([Details](./intent-engine))

Describe network changes in natural language. Lattice produces a structured CRD change plan for review.

- Two-stage LLM pipeline: structured intent extraction → human-readable diff
- `POST /api/v1/ai/intent/plan` — preview changes without applying
- `POST /api/v1/ai/intent/apply` — execute approved plans through WorkflowService
- Risk level assessment before any change is applied

## Layer 4 — Time-Travel Network Debugging (Pro) ([Details](./debugging))

Automatic network state snapshots let you debug any past state with AI assistance.

- Snapshots triggered by policy changes, peer online/offline, manual, or scheduled
- Diff any two snapshots to see exactly what changed
- AI-powered root cause analysis: ask questions about past state
- Get/snapshot/diff MCP tools for Claude-assisted debugging

## Layer 5 — Compliance-as-Conversation (Pro) ([Details](./compliance))

Generate compliance reports and evidence packages from your Lattice network state.

- Framework support: SOC2 Type II, PCI-DSS, HIPAA
- Automated control verification against network policies and change history
- Downloadable evidence packages (ZIP) with SHA256 attestation
- Executive summaries generated by AI for CISO review

## Getting Started

1. [Enable the AI module](../config/reference#ai) in your `lattice.yaml`
2. Configure your LLM provider (Anthropic or OpenAI-compatible)
3. Connect via the [MCP Server](./mcp-server) or use the built-in dashboard Chat interface

See the [MCP Server setup guide](./mcp-server) for Claude Desktop / Claude Code integration.
```

- [ ] **Step 2: 更新 ai/agent-enrollment.md 顶部说明**

在 `docs/ai/agent-enrollment.md` 文件顶部的 `# Zero-Trust AI Agent Networking` 标题下方（第 7 行后）插入一个说明框：

在文件开头（frontmatter 之前）增加 frontmatter，并在 h1 标题后加说明框。

用以下内容完整替换 `docs/ai/agent-enrollment.md`：

```markdown
---
title: Agent Enrollment API
---

# Agent Enrollment API

::: info Two ways to give AI agents a Lattice identity
**This page covers the HTTP Enrollment API** — agents call `POST /api/v1/agent-enroll` and receive a WireGuard config.

For the **CLI sandbox** (`lattice sandbox start`) — which uses gVisor user-space isolation and requires zero privileges — see [Agent Platform → Sandbox](/agent/sandbox).
:::

AI agents introduce a new security threat: a compromised agent can **lateral move** across your infrastructure after a prompt injection attack. Lattice solves this at the network layer with WireGuard + Policy — **every agent gets its own identity with time-bound, network-isolated access.**

## Agent Enrollment API

Agents self-register via a single API call:

```http
POST /api/v1/agent-enroll
```

**Request:**

```json
{
  "agentName": "code-executor-001",
  "agentType": "code-executor",
  "workspaceId": "ws-prod-agents",
  "ttl": "1h",
  "policyPreset": "sandboxed"
}
```

**Response:**

```json
{
  "peerId": "peer-xxx",
  "overlayIP": "10.96.2.5/32",
  "enrollmentToken": "lt-xxx",
  "wireguardConfig": "...",
  "expiresAt": "2026-05-06T11:00:00Z"
}
```

## Policy Presets

| Preset | Rules |
|--------|-------|
| `sandboxed` | Egress-only to designated tool services, deny all ingress |
| `coordinator` | Accepts ingress from same-workspace agents |
| `isolated` | Full isolation, allowlisted IP/port only |

## TTL Auto-Destruction

- LatticePeer gets an `ExpiresAt` annotation on creation
- Manager reconciler scans for expired peers every minute and deletes them automatically
- Agents can proactively call `DELETE /api/v1/peers/:id` on graceful shutdown (wrapped in SDK)

## SDK Integration

### Python SDK

```python
from lattice_sdk import LatticeAgent

async with LatticeAgent(
    server="https://lattice.company.com",
    token="lt-workspace-token",
    agent_name="code-executor",
    policy_preset="sandboxed",
) as agent:
    result = await my_agent_task()
```

The SDK handles enrollment, WireGuard config setup, TTL renewal, and graceful shutdown.

### Framework Integration

| Framework | Integration Point |
|-----------|------------------|
| LangGraph | `StateGraph` lifespan context manager |
| AutoGen | `ConversableAgent` init/del hooks |
| Claude Agent SDK | Agent startup script wrapper |
| Kubernetes Job | Init container enroll + sidecar heartbeat |

## Why Zero-Trust Networking for AI Agents?

Without network-level isolation, a prompt injection attack on any agent gives attackers access to the **entire internal network**. Lattice's approach ensures:

- Each agent has a unique, cryptographically verified identity (WireGuard public key)
- Network policy is enforced at the kernel level (iptables/eBPF), not in application code
- Identity is time-bound — even if a key is compromised, it expires automatically
- Lateral movement is blocked by default-deny network policies
```

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/ai/index.md docs/ai/agent-enrollment.md
git commit -s -m "docs(ai): clarify Layer 2 methods, distinguish HTTP API from CLI sandbox"
```

---

### Task 7: 更新 guide/quickstart.md 和 LatticeSandbox.vue

**Files:**
- Modify: `docs/guide/quickstart.md`
- Modify: `docs/.vitepress/theme/components/LatticeSandbox.vue`

- [ ] **Step 1: 更新 LatticeSandbox.vue 演示内容**

找到 `LatticeSandbox.vue` 中的 `lines` 数组（第 33-47 行），用以下内容替换：

```typescript
  const lines = [
    '$ lattice --version\r\n',
    'lattice v0.3.0\r\n\n',
    '$ latticed start --dev\r\n',
    'INF Starting LatticeD all-in-one...\r\n',
    'INF NATS server started on :4222\r\n',
    'INF Web UI available at http://localhost:8080\r\n\n',
    '$ lattice sandbox start --name agent-001 --token lt_demo\r\n',
    'INF Registering sandbox via NATS name=agent-001\r\n',
    'INF gVisor sandbox initialized localIP=10.42.0.5\r\n',
    'INF ICE tunnel established peer=10.42.0.1\r\n',
    'INF Tunnel status: READY\r\n\n',
    '$ lattice policy create allow-tools --port 443 --target app=tools\r\n',
    'INF Policy "allow-tools" created\r\n',
    'INF Policy active on 2 nodes\r\n',
  ]
```

并同时更新 sandbox header title（第 90 行附近）：

```html
      <span class="title">Lattice Sandbox — lattice sandbox start</span>
```

- [ ] **Step 2: 验证 quickstart.md 命令正确性**

查看 `docs/guide/quickstart.md`，确认 `lattice init` 和 `lattice up` 命令是否仍然有效（与当前 CLI 一致）。如果发现版本引用，更新为当前版本。当前文件中无硬编码版本号，无需修改。

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/.vitepress/theme/components/LatticeSandbox.vue
git commit -s -m "docs(theme): update terminal demo to show sandbox workflow"
```

---

### Task 8: 新建 design/architecture.md 和 design/sandbox.md

**Files:**
- Create: `docs/design/architecture.md`
- Create: `docs/design/sandbox.md`

- [ ] **Step 1: 创建 design 目录并写入 architecture.md**

```bash
mkdir -p /Users/francis/workspc/lattice/docs/design
```

写入 `docs/design/architecture.md`：

```markdown
---
title: Architecture Overview
---

# Architecture Overview

Lattice is composed of three core components:

| Component | Entry Point | Purpose |
|-----------|-------------|---------|
| **Lattice Agent** | `cmd/lattice` | Edge node: WireGuard tunnel, NATS signaling, ICE/LRP |
| **LatticeD** | `cmd/latticed` | All-in-one control plane: NATS + SQLite + API + UI |
| **Manager** | `cmd/manager` | Kubernetes operator: reconciles CRDs |

## Transport Layer

Each peer-to-peer connection goes through a state machine:

```
Created → Probing → ICEReady / LRPReady → Failed → Closed
```

ICE (direct P2P) and LRP (relay fallback) race in parallel. Whichever establishes first wins. The connection upgrades from LRP to ICE transparently when a direct path becomes available.

```
Peer A                                    Peer B
  │  ── NATS signal ──────────────────────▶  │
  │  ◀── NATS signal ──────────────────────  │
  │                                          │
  │  ── ICE probe ─────────────────────────▶ │  ← races with LRP
  │  ── LRP connect ───────────▶ Relay ────▶ │
  │                                          │
  └─────────── WireGuard data ───────────────┘
```

## Policy Enforcement

`PolicyEnforcer` interface in `internal/agent/provision/provisioner.go`:

```
SelectEnforcerMode()
  ├── Linux 5.10+ && EDITION=pro → eBPF TC on wf0 TUN interface
  └── default                   → iptables
```

Community edition always uses iptables. Pro edition uses eBPF when kernel supports it, with automatic fallback to iptables.

## Agent Sandbox

The sandbox provides a **zero-privilege** alternative to the regular node:

- Regular node: kernel TUN (`wf0`), requires `CAP_NET_ADMIN`
- Sandbox: gVisor `pkg/tcpip` + `TUNAdapter`, runs as any user

See [Sandbox Architecture](/design/sandbox) for the detailed design.

## Signal Flow

```
LatticeD (NATS broker)
    │
    ├── lattice.signals.peers.<PeerID>  ← peer signaling
    ├── lattice.audit.flow              ← Pro: NATS flow audit
    └── lattice.config.<PeerID>         ← peer config push
```
```

- [ ] **Step 2: 创建 design/sandbox.md（整理自 specs）**

写入 `docs/design/sandbox.md`：

```markdown
---
title: Sandbox Architecture
---

# Sandbox Architecture

> Source: `cmd/lattice/cmd/sandbox/`, `internal/agent/gvisor/`

## Overview

`lattice sandbox start` fuses the **gVisor user-space network stack** with the **Lattice WireGuard overlay**. AI agent processes run as ordinary users with no kernel capabilities, yet obtain a full Lattice network identity — NATS registration, ICE hole-punching, LRP relay fallback — identical to a regular node.

## Comparison with Regular Node

| Dimension | Regular Node (`lattice up`) | Sandbox (`lattice sandbox start`) |
|-----------|---------------------------|----------------------------------|
| Isolation | None (host process) | gVisor user-space netstack |
| Privilege | root / `CAP_NET_ADMIN` | **Zero-privilege** |
| Network stack | Kernel TUN (`wf0`) | gVisor `pkg/tcpip` + TUNAdapter |
| WireGuard | Kernel `wgctrl` | `golang.zx2c4.com/wireguard` (user-space) |
| Provisioner | `KernelProvisioner` (iptables/eBPF) | `SandboxProvisioner` (no iptables) |
| Registration | HTTP or NATS | NATS only (`RegisterSandboxViaNATS`) |
| Credential persistence | None | JSON file (`/etc/lattice/sandbox-credentials.json`) |
| Audit log | eBPF ring buffer (Pro) | JSONL file (`/tmp/lattice-audit-<name>.jsonl`) |
| Egress policy | eBPF TC (Pro) / iptables | `EgressFilter` (Pro sandbox only) |
| Inbound forwarding | None | `ForwardListener` (Pro) |
| HTTP proxy | None | HTTP forward proxy (Pro) |
| ICE / LRP | ✅ Full support | ✅ Full support (shared infrastructure) |

## Network Architecture

```
                ┌─────────────────────────────┐
                │       gVisor Sandbox         │
                │                              │
  Agent process ──▶  gVisor netstack (tcpip)   │
  connect()         │                          │
                │  [Pro] EgressFilter          │
                │   │                          │
                │   TUNAdapter (channel bridge)│
                │   │                          │
                │   wireguard-go Device        │
                └──────────┬───────────────────┘
                           │ UDP :51820
                ┌──────────▼───────────────────┐
                │   FilteringUDPMux             │
                │   STUN ──▶ ICE agent          │
                │   non-STUN ──▶ WG DefaultBind │
                └──────────┬───────────────────┘
                           │
          ┌────────────────┴──────────────┐
          │  ICE succeeds                 │  ICE fails
          ▼                               ▼
    Direct P2P                    LRP relay (QUIC/TCP)
```

The sandbox uses the **same signaling path** as a regular node. gVisor replaces only the kernel TUN device.

## Code Structure

```
cmd/lattice/cmd/sandbox/
├── sandbox.go              # Command definition (--name, --server-url, --token)
├── sandbox_shared.go       # No build tag — shared utilities (credential I/O, fileAuditWriter)
├── sandbox_community.go    # //go:build !pro — full community implementation
└── sandbox_pro.go          # //go:build pro  — Pro-only extensions

internal/agent/gvisor/
├── sandbox.go              # gvisor.New() entry point, Config{ID, LocalIP, PolicyChecker, AuditWriter}
├── tun_adapter.go          # NewTUNAdapter: gVisor ↔ wireguard-go packet bridge
├── provisioner.go          # SandboxProvisioner (no iptables, replaces KernelProvisioner)
└── shimfwd/
    ├── egress_filter.go    # EgressFilter (CIDR allowlist/denylist, implements PolicyChecker)
    ├── forward_listener.go # ForwardListener (overlay port → host address forwarding)
    └── audit_writer.go     # AuditWriter interface + AuditEvent struct
```

## Community vs Pro Build Tags

```go
// sandbox_community.go
//go:build !pro

// sandbox_pro.go
//go:build pro
```

Community stubs for Pro features return `"... is a Pro feature"` errors. Build with `make EDITION=pro build` to include Pro code.
```

- [ ] **Step 3: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/design/architecture.md docs/design/sandbox.md
git commit -s -m "docs(design): add architecture overview and sandbox architecture pages"
```

---

### Task 9: 新建 design/ice-connection.md、ice-wireguard-mux.md、wrrp-quic.md

**Files:**
- Create: `docs/design/ice-connection.md`
- Create: `docs/design/ice-wireguard-mux.md`
- Create: `docs/design/wrrp-quic.md`

- [ ] **Step 1: 创建 design/ice-connection.md（整理自 specs）**

写入 `docs/design/ice-connection.md`：

```markdown
---
title: ICE Connection & LRP Relay
---

# ICE Connection & LRP Relay

> Source: `superpowers/specs/2026-05-01-ice-relay-wireguard-design.md`

## Motivation

Lattice connects nodes across arbitrary network topologies — home LANs, corporate firewalls, cloud VPCs. A WireGuard mesh alone fails behind symmetric NAT. The transport layer addresses this with a **dual-path strategy**: direct P2P via ICE when possible, relay fallback via LRP when NAT traversal fails.

Design priorities:
1. **Lowest latency wins**: ICE direct path preferred; LRP is the fallback
2. **Seamless upgrade**: LRP → ICE upgrade happens transparently mid-connection
3. **Single port**: WireGuard and ICE share UDP port 51820 via a filtering mux
4. **Multi-transport relay**: LRP supports both TCP (HTTP upgrade) and QUIC (datagram)

## Architecture

```
          Peer A                                  Peer B
      ┌───────────┐                          ┌───────────┐
      │   wf0     │                          │   wf0     │
      │   TUN     │                          │   TUN     │
      └─────┬─────┘                          └─────┬─────┘
            │                                      │
      ┌─────▼─────┐                          ┌─────▼─────┐
      │ WireGuard │                          │ WireGuard │
      │  Device   │                          │  Device   │
      └─────┬─────┘                          └─────┬─────┘
            │                                      │
  ┌─────────▼─────────┐                  ┌─────────▼─────────┐
  │   FilteringUDPMux  │                  │   FilteringUDPMux  │
  │   :51820           │                  │   :51820           │
  └──┬─────────────┬──┘                  └──┬─────────────┬──┘
     │             │                        │             │
┌────▼────┐  ┌─────▼──────┐          ┌────▼────┐  ┌─────▼──────┐
│ ICE     │  │ LRP Client │          │ ICE     │  │ LRP Client │
│ Agent   │  │ (TCP/QUIC) │          │ Agent   │  │ (TCP/QUIC) │
└─────────┘  └─────┬──────┘          └─────────┘  └─────┬──────┘
                   └──────── Relay Server ────────────────┘

     Signaling: NATS (lattice.signals.peers.<PeerID>)
```

## ICE / NAT Traversal

Built on [pion/ice v4](https://github.com/pion/ice).

- STUN servers for reflexive address discovery
- TURN servers for relay candidates (fallback within ICE)
- `FilteringUDPMux` demultiplexes STUN and WireGuard packets on the same port

## LRP Relay

LRP (Lattice Relay Protocol) is the outer relay when ICE fails (e.g., symmetric NAT on both sides):

- TCP transport: HTTP-upgraded persistent connection
- QUIC transport: datagram-based, eliminates head-of-line blocking

## Connection State Machine

```
Created → Probing → ICEReady / LRPReady → Failed → Closed
```

ICE and LRP probe in parallel. First to succeed wins. The state machine handles upgrade (LRP → ICE) and fallback transparently.
```

- [ ] **Step 2: 创建 design/ice-wireguard-mux.md**

写入 `docs/design/ice-wireguard-mux.md`：

```markdown
---
title: ICE + WireGuard Shared Port
---

# ICE + WireGuard Shared Port

> Source: `superpowers/specs/ice-wireguard-mux.md`

## Problem

Lattice uses a single UDP port `:51820` for both ICE signaling (STUN) and WireGuard data traffic. Two goroutines compete for packets on the same `net.UDPConn`:

| Packet type | Taken by `connWorker` | Taken by `makeReceiveIPv4` |
|-------------|----------------------|---------------------------|
| STUN | ✅ correct → ICE | ❌ WireGuard decrypt fails, dropped |
| WireGuard encrypted | ❌ no ufrag match, dropped | ✅ correct |

## Solution: FilteringUDPMux

`FilteringUDPMux` is a custom `UDPMux` that inspects each incoming packet before dispatching:

1. **STUN packet** (starts with `0x00` or `0x01`, magic cookie `0x2112A442`) → dispatch to ICE agent
2. **Non-STUN packet** → dispatch to WireGuard `DefaultBind`

This eliminates the race condition. The ICE agent and WireGuard device each see only their own traffic.

## ICE Agent Lifecycle

After `agent.Dial()` / `Accept()` succeeds:
- ICE agent is closed — `connWorker` goroutine stops
- WireGuard's `PersistentKeepalive` maintains the NAT mapping
- STUN keepalive is not needed and is eliminated

## Implementation

```
internal/agent/transport/
└── filtering_mux.go   # FilteringUDPMux implementation
```

Key function: `isSTUN(buf []byte) bool` — checks the STUN magic cookie at bytes 4-7.
```

- [ ] **Step 3: 创建 design/wrrp-quic.md**

写入 `docs/design/wrrp-quic.md`：

```markdown
---
title: WRRP / QUIC Relay Transport
---

# WRRP / QUIC Relay Transport

> Source: `superpowers/specs/design-wrrp-quic.md`

## Background

WRRP (Wireflow Relay & Routing Protocol) is the relay channel used when two peers cannot establish a direct ICE path. The original implementation tunnels WireGuard packets over a persistent HTTP-upgraded TCP connection.

### Problems with TCP relay

| Problem | Impact |
|---------|--------|
| Head-of-Line blocking | A dropped TCP segment stalls all peers on the same connection |
| Application-level keepalive | Custom Ping/Pong logic required |
| Slow reconnect | TCP + HTTP handshake on every reconnect |
| No packet-boundary preservation | Framing layer required on top of TCP stream |

## Goals

1. Eliminate HoL blocking between peer pairs sharing a relay connection
2. Simplify keepalive — delegate to QUIC's built-in mechanism
3. Faster reconnect — QUIC 0-RTT resumption
4. Preserve packet boundaries — QUIC datagrams map 1:1 to WireGuard packets
5. Backward compatibility — TCP relay path remains fully operational; QUIC is opt-in

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    WRRP Relay Server                     │
│                                                          │
│  ┌─────────────────────┐   ┌─────────────────────────┐  │
│  │   TCP Server        │   │   QUIC Server           │  │
│  │   :6266 (HTTP upg.) │   │   :6267 (UDP)           │  │
│  └────────┬────────────┘   └──────────┬──────────────┘  │
│           └──────────┐   ┌────────────┘                  │
│                      ▼   ▼                               │
│               ┌─────────────────┐                        │
│               │  WRRPManager    │                        │
│               │  streams map    │  ← TCP sessions        │
│               │  quicConns map  │  ← QUIC sessions       │
│               └────────┬────────┘                        │
│                        │ Relay(toID, frame)              │
│                        │  • QUIC: SendDatagram           │
│                        │  • TCP:  Stream.Write           │
└────────────────────────┼─────────────────────────────────┘
                         │
         ┌───────────────┴───────────────┐
         ▼                               ▼
  ┌─────────────┐                 ┌─────────────┐
  │  Agent A    │                 │  Agent B    │
  │  QUIC client│                 │  QUIC client│
  └─────────────┘                 └─────────────┘
```

## Non-Goals

- **P2P QUIC between peers** — QUIC cannot replace ICE for NAT traversal. ICE handles hole-punching; QUIC cannot.
- **TLS PKI** — Relay server uses a self-signed certificate at startup. Clients use `InsecureSkipVerify`. Underlying WireGuard encryption provides end-to-end security.
```

- [ ] **Step 4: Commit**

```bash
cd /Users/francis/workspc/lattice && git add docs/design/ice-connection.md docs/design/ice-wireguard-mux.md docs/design/wrrp-quic.md
git commit -s -m "docs(design): add ICE connection, WireGuard mux, and WRRP/QUIC design pages"
```

---

### Task 10: 构建验证

**Files:** none (read-only verification)

- [ ] **Step 1: 运行 VitePress 构建**

```bash
cd /Users/francis/workspc/lattice/docs && pnpm docs:build 2>&1 | tail -30
```

Expected: `build complete` 无 error，dead link warning 只应来自 `ignoreDeadLinks` 已豁免的路径（`/demo/`、`/features/`、`localhost`）。

- [ ] **Step 2: 检查新增路径无死链**

```bash
cd /Users/francis/workspc/lattice/docs && pnpm docs:build 2>&1 | grep -i "dead link\|error" | grep -v "features\|demo\|localhost" || echo "No unexpected dead links"
```

Expected: `No unexpected dead links`

- [ ] **Step 3: 如有死链，修复后重新 commit**

常见问题：
- `config.mts` 中 `agentSidebar()` 引用了 `/agent/sandbox-pro` → 确认文件名为 `sandbox-pro.md`
- `agent/index.md` 中链接 `/agent/sandbox` → 确认文件存在

- [ ] **Step 4: 最终 commit（如有修复）**

```bash
cd /Users/francis/workspc/lattice && git add -A docs/
git commit -s -m "docs(fix): resolve dead links found in build verification"
```
