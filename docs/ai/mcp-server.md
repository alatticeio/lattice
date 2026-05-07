---
title: MCP Server + AI ChatOps
---

# MCP Server + AI ChatOps

Lattice implements the **Model Context Protocol (MCP)** to expose network management capabilities to AI assistants. Any MCP client — Claude Desktop, Claude Code, Cursor — can manage your Lattice network directly.

## Architecture

```
Claude Desktop / Claude Code / Cursor / any MCP client
          ↓  MCP protocol (JSON-RPC 2.0 over stdio / HTTP)
    lattice-mcp
          ↓  REST API
    Lattice Management Server
```

The `lattice-mcp` binary is a standalone process. It communicates with the Lattice Management Server via REST API and does not require direct K8s API access.

## Tools

### Read Tools

| Tool | Description |
|------|-------------|
| `list_peers` | List all peers in a workspace with status |
| `list_policies` | List all network policies |
| `list_networks` | List all networks with CIDR and peer count |
| `check_connectivity` | Simulate connectivity check between two peers |
| `audit_workspace` | Run a security audit on a workspace |

### Write Tools

Write tools require approval via `WorkflowService` by default:

| Tool | Description |
|------|-------------|
| `create_peer` | Create a new LatticePeer + enrollment token |
| `delete_peer` | Delete a peer with NATS disconnect notification |
| `create_policy` | Create a new LatticePolicy |
| `update_policy` | Modify an existing policy |
| `delete_policy` | Delete a policy |

## Write Tool Security Model

Write operations follow a configurable approval flow:

```
MCP calls write tool
    |
    +- auto_approve=true  → execute immediately
    +- auto_approve=false → WorkflowService.Submit()
           → returns workflow_id, waits for human approval
           → after approval: WorkflowService.Approve() → K8s Apply
```

Configure per-tool auto-approve in `lattice.yaml`:

```yaml
ai:
  enabled: true
  api-key: "sk-..."
  provider: anthropic   # anthropic | openai-compat
  workflow:
    auto_approve:
      create_peer: false
      delete_peer: false
      create_policy: false
```

## Setup

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "lattice": {
      "command": "lattice-mcp",
      "args": ["--config", "/etc/lattice/lattice.yaml"]
    }
  }
}
```

### Claude Code

Add to your project's `.claude/mcp.json`:

```json
{
  "mcpServers": {
    "lattice": {
      "command": "lattice-mcp",
      "args": ["--config", "/etc/lattice/lattice.yaml"]
    }
  }
}
```

### Transport Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| stdio | Subprocess communication via stdin/stdout | Claude Desktop / Code (default) |
| SSE | HTTP-based transport for remote access | Team-shared MCP server |

## Design Principles

- **MCP is the unified entry point** — Claude Desktop, Claude Code, Cursor, and custom agents all connect through the same protocol
- **CRD as Source of Truth** — all state reads and writes go through K8s API
- **Human-in-the-loop** — write operations go through WorkflowService approval by default, with `auto_approve` configurable per tool
- **Tool logic shared with built-in AI** — the same backend tools power both the dashboard Chat UI and the MCP server
