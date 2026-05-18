<div align="center">

# Lattice

**Self-Hosted WireGuard Mesh · AI Agent Sandbox**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/alatticeio/lattice)](https://goreportcard.com/report/github.com/alatticeio/lattice)
[![Release](https://img.shields.io/github/v/release/alatticeio/lattice)](https://github.com/alatticeio/lattice/releases/latest)
[![Container](https://img.shields.io/badge/ghcr.io-alatticeio%2Flattice-blue)](https://github.com/alatticeio/lattice/pkgs/container/lattice)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[![Handshake LAN](https://img.shields.io/endpoint?url=https://alatticeio.github.io/lattice/docs/benchmarks/handshake.json)](https://github.com/alatticeio/lattice/actions/workflows/bench.yml)
[![Throughput](https://img.shields.io/endpoint?url=https://alatticeio.github.io/lattice/docs/benchmarks/throughput.json)](https://github.com/alatticeio/lattice/actions/workflows/bench.yml)
[![API p99](https://img.shields.io/endpoint?url=https://alatticeio.github.io/lattice/docs/benchmarks/api-p99.json)](https://github.com/alatticeio/lattice/actions/workflows/bench.yml)

Lattice is a self-hosted platform built around **two core pillars**: a **network orchestration engine** that connects any device — servers, containers, IoT, Kubernetes pods — into an encrypted WireGuard overlay mesh, and an **AI agent sandbox** that gives every AI agent a zero-trust WireGuard identity with kernel-level isolation and natural-language policy management.

[**Website**](https://alattice.io) · [**Documentation**](https://alattice.io/docs) · [**Issues**](https://github.com/alatticeio/lattice/issues)

</div>

---

## Two Core Pillars

### Network Orchestration

Connect any device into an encrypted overlay — no firewall changes, no public IP exposure.

| Capability | Description |
|------------|-------------|
| **WireGuard Tunnel Automation** | Key distribution, rotation, and peer discovery are fully automated; first-handshake time (TTFH) is observable |
| **NAT Traversal** | Dual-stack ICE/STUN (IPv4 + IPv6), concurrent LRP relay fallback, works across symmetric NAT |
| **Built-in IPAM** | Two-tier allocation (global pool → subnet → peer IP), optimistic locking, automatic hole reuse |
| **Policy Engine** | Default-deny + label selectors + port-level ingress/egress; TTL-based expiry; dual backend: iptables (Community) / eBPF TC (PRO) |
| **Multi-Workspace & RBAC** | Namespace isolation + cross-workspace peering + cross-cluster peering + invitations |
| **LRP Relay** | Custom relay protocol with TCP + QUIC dual transport, automatic failover when P2P is unavailable |
| **K8s Operator** | 13 CRDs (Network, Peer, Policy, IPPool, Relay, Peering, etc.) for declarative lifecycle management |
| **Web Dashboard** | Visual topology, policy editor, monitoring dashboard, workspace management |
| **Telemetry** | PRO: VictoriaMetrics push (system metrics, per-peer WireGuard traffic/latency/packet loss) |
| **All-in-One Deployment** | Embedded NATS + SQLite, zero external dependencies, one `docker run` to start |

### AI Agent Sandbox

Give every AI agent a secure network identity — kernel-level isolation, natural-language-driven policy changes.

| Capability | Description |
|------------|-------------|
| **AgentIdentity CRD** | Binds an AI agent to a WireGuard Peer with RBAC (AllowedTools, AllowedNamespaces); four SandboxModes (none/pod/gvisor/microvm) and three EnforcementModes (disabled/audit/enforce) |
| **Zero-Trust Enrollment** | Single-use Enrollment Token (TTL + usage limit) → auto-create LatticePeer + AgentIdentity → issue JWT — no manual key setup |
| **Agent Isolation Enforcement** | `ExecuteTool()` path enforces: is the identity expired/revoked? is the namespace whitelisted? is the tool whitelisted? Audit mode logs violations; enforce mode blocks them |
| **Agent JWT Auth Middleware** | HS256-signed, 365-day expiry, injected into Gin context; human users and agents share the same API, context auto-detects the caller |
| **gVisor Sandbox** | `lattice sandbox start` CLI + `internal/agent/gvisor/` runtime: user-space netstack (pkg/tcpip), zero privileges, no TUN, no eBPF; full ICE/LRP peer connectivity shared with regular agents; Community: network isolation + local audit; PRO: adds egress policy, port forwarding, HTTP proxy, centralized NATS audit |
| **MCP Server** | `lattice-mcp` binary for Claude Desktop / Cursor; 14 tools (read: list_peers, list_policies, check_connectivity, etc.; write: create_policy, delete_peer, etc. with human approval) |
| **Intent Engine (PRO)** | Natural language → LLM extracts CRD change plan → Markdown diff preview → approve → apply — full human-in-the-loop workflow |
| **Tool Call Audit & Trace** | Every agent tool call records a `tool_spans` entry (traceID, agentID, tool, status, durationMs); query via `GET /api/v1/agent-isolation/audit/traces`; PRO: gVisor flow events linked to traces |
| **Sub-agent Delegation** | Parent agent calls `POST /api/v1/agent-isolation/delegate` to spawn a child agent with scoped tool permissions; child registers independently with its own WireGuard identity; call tree queryable via API |

---

## Comparison

### Network Orchestration

| Capability | Lattice | Tailscale | Netbird | ZeroTier |
|------------|---------|-----------|---------|----------|
| Self-hosted control plane | ✅ | ❌ (SaaS only) | ✅ | ✅ |
| Web Dashboard | ✅ | ✅ | ✅ | ✅ |
| K8s CRD Operator | ✅ (13 CRDs) | ✅ (limited) | ❌ | ❌ |
| eBPF policy enforcement | ✅ (PRO) | ❌ | ❌ | ❌ |
| Policy TTL expiry | ✅ | ❌ | ❌ | ❌ |
| Cross-workspace peering | ✅ | ❌ | ❌ | ❌ |
| Built-in IPAM | ✅ | ✅ | ❌ | ❌ |

### AI Agent Sandbox

| Capability | Lattice | Tailscale | Netbird | ZeroTier |
|------------|---------|-----------|---------|----------|
| Agent zero-trust enrollment (TTL + network isolation presets) | ✅ | ❌ | ❌ | ❌ |
| AgentIdentity CRD + RBAC | ✅ | ❌ | ❌ | ❌ |
| gVisor user-space kernel sandbox | ✅ (Community + PRO) | ❌ | ❌ | ❌ |
| MCP Server (AI assistant manages network via natural language) | ✅ | ❌ | ❌ | ❌ |
| Intent Engine (natural language → CRD → approve → apply) | ✅ (PRO) | ❌ | ❌ | ❌ |
| Tool call audit logging | ✅ | ❌ | ❌ | ❌ |
| Write-op approval workflow | ✅ | N/A | N/A | N/A |
| Sidecar intent interception (seccomp notify) | 🔜 | ❌ | ❌ | ❌ |
| eBPF PID ↔ TUN traffic binding | 🔜 | ❌ | ❌ | ❌ |

---

## Architecture

![Architecture](docs/images/architecture.png)

Lattice consists of four planes:

- **Control Plane** — K8s Operator or All-in-One standalone mode (`latticed`), declaratively managing network topology, keys, IP allocation, and peer relationships
- **Data Plane** — Lightweight agent (~12 MB) deployed on any device, establishing encrypted WireGuard tunnels with ICE/STUN NAT traversal
- **Relay Plane** — Custom LRP relay protocol, automatic fallback when direct P2P is unavailable
- **Sandbox Plane** — gVisor user-space kernel + Agent JWT + tool-level RBAC, providing a zero-privilege execution environment for AI agents

---

## Quick Start

### Docker (single command, no Kubernetes required)

```bash
docker run -d \
  --name lattice-k3s \
  --privileged \
  -p 8080:8080 \
  ghcr.io/alatticeio/lattice-k3s:latest
```

This container bundles k3s + the Lattice control plane. After ~30 seconds, visit `http://localhost:8080`.

### Existing Kubernetes cluster

```bash
kubectl apply -k https://github.com/alatticeio/lattice/config/lattice/overlays/all-in-one
```

---

## Connecting a Device

### 1. One-time setup

```bash
lattice init
```

Follow the prompts to enter your Server URL and Enrollment Token. Config is saved to `~/.lattice/lattice.yaml`.

### 2. Create a workspace

```bash
lattice workspace add dev --display-name "Development"
```

### 3. Create an enrollment token

```bash
lattice token create dev-team -n <namespace> --limit 10 --expiry 168h
```

### 4. Start the agent

```bash
lattice up
```

### 5. Allow traffic (default-deny)

```bash
lattice policy allow-all -n <namespace>
```

### 6. Verify

```bash
lattice status     # Show local WireGuard status and peer list
ping 10.100.0.2    # Ping a peer to confirm the tunnel is up
```

---

## AI Agent Sandbox

### CLI: Start a sandboxed agent

```bash
# Community: gVisor network isolation + local audit
lattice sandbox start \
  --name my-agent \
  --server-url https://lattice.company.com \
  --token lt-enroll-xxx

# PRO: adds egress policy, inbound port forwarding, HTTP proxy
lattice sandbox start \
  --name my-agent \
  --server-url https://lattice.company.com \
  --token lt-enroll-xxx \
  --egress-allow 10.100.0.0/24 \
  --egress-default-deny \
  --forward 8080:127.0.0.1:8080 \
  --proxy-addr 127.0.0.1:1080
```

On start, the sandbox completes zero-trust enrollment via NATS (generate WireGuard keypair → register with enrollment token → receive VPN IP → connect via ICE/LRP), then runs as a full Lattice overlay node backed by gVisor's user-space network stack. No root, no TUN device, no eBPF required. Credentials are persisted across container restarts.

### REST API: Agent enrollment and management

```bash
# Create an enrollment token
curl -X POST https://lattice.company.com/api/v1/agent-isolation/enrollment-tokens \
  -H "Authorization: Bearer $HUMAN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"namespace":"default","allowedTools":["list_peers","check_connectivity"],"ttlSeconds":3600}'

# Revoke an agent
curl -X DELETE "https://lattice.company.com/api/v1/agent-isolation/agents/code-executor?namespace=default" \
  -H "Authorization: Bearer $HUMAN_TOKEN"

# Delegate a sub-agent (parent agent spawns a child with scoped permissions)
curl -X POST https://lattice.company.com/api/v1/agent-isolation/delegate \
  -H "Authorization: Bearer $AGENT_JWT" \
  -H "Content-Type: application/json" \
  -d '{"agentName":"sub-executor","requestedTools":["exec","read"],"ttlSeconds":900}'

# Query tool call traces
curl "https://lattice.company.com/api/v1/agent-isolation/audit/traces?agentId=my-agent" \
  -H "Authorization: Bearer $HUMAN_TOKEN"
```

---

## AI Assistant Integration (MCP)

```bash
go install github.com/alatticeio/lattice/cmd/lattice-mcp@latest
```

Add to Claude Desktop (`~/.config/claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "lattice": {
      "command": "lattice-mcp",
      "args": ["--workspace", "YOUR_WORKSPACE_ID"]
    }
  }
}
```

Then ask Claude in natural language: "List all peers", "Create a policy allowing frontend to reach api-gateway on port 443", "Why can't payment-service reach postgres?"

---

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap alatticeio/tap
brew install lattice
```

### APT (Debian / Ubuntu)

```bash
echo "deb [trusted=yes] https://alatticeio.github.io/lattice/deb ./" | sudo tee /etc/apt/sources.list.d/lattice.list
sudo apt update && sudo apt install lattice
```

### YUM (RHEL / CentOS / Fedora)

```bash
sudo tee /etc/yum.repos.d/lattice.repo <<< '[lattice]
name=Lattice
baseurl=https://alatticeio.github.io/lattice/rpm
enabled=1
gpgcheck=0'
sudo yum install lattice
```

### Binary

```bash
VERSION=$(curl -s https://api.github.com/repos/alatticeio/lattice/releases/latest | grep tag_name | cut -d'"' -f4)
curl -sSL "https://github.com/alatticeio/lattice/releases/download/${VERSION}/lattice_${VERSION}_linux_amd64.tar.gz" | tar xz
sudo mv lattice /usr/local/bin/
```

---

## Development

```bash
git clone https://github.com/alatticeio/lattice.git
cd lattice
make build-all     # Build all binaries
make test          # Run unit tests
make lint          # Run golangci-lint
```

Requirements: Go 1.25+ / Docker 20.10+ / k3d 5.x+ (E2E) / kubectl 1.20+

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a pull request.

<a href="https://github.com/alatticeio/lattice/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=alatticeio/lattice" />
</a>

---

## Disclaimer

This tool is intended for legitimate technical research, enterprise private networking, and compliant remote access scenarios only. Users are responsible for ensuring their use complies with all applicable local laws and regulations. The authors assume no liability for any misuse of this software.

## License

[Apache License 2.0](LICENSE)
