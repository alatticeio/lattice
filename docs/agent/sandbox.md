---
title: Agent Sandbox (Community)
---

# Agent Sandbox — Community Edition

`lattice sandbox start` is a zero-privilege sandbox command built into the main `lattice` CLI. It supports two isolation modes:

- **pod mode** (`--mode pod`, default): embeds a gVisor user-space network stack in-process, with an optional SOCKS5 proxy for AI agent traffic.
- **gvisor mode** (`--mode gvisor`, [Pro](/agent/sandbox-pro)): runs the AI agent inside a gVisor runsc container with syscall-level isolation. See [Pro sandbox docs](/agent/sandbox-pro) for details.

## Network Architecture (pod mode)

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

## Quick Start (pod mode)

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

Flags (Community):
  --name         string   Sandbox identity name (required)
  --server-url   string   LatticeD URL (default: http://localhost:8080)
  --token        string   Enrollment token (required)
  --mode         string   Isolation mode: pod (default) | gvisor (Pro only)
```

Pro-only flags: see [Agent Sandbox (Pro)](/agent/sandbox-pro).

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
