# Zero-Trust AI Agent Enrollment

Auto-enroll Claude Desktop, Cursor, or custom AI agents with time-bound WireGuard identities.

## Overview

Lattice can automatically issue ephemeral WireGuard credentials to AI agents. Each agent gets a scoped, time-limited identity — no permanent keys, no over-privileged access.

## Step 1: Enable MCP Server

Ensure the MCP Server is running in your Lattice deployment. See [MCP Server & ChatOps](/ai/mcp-server).

## Step 2: Configure AI Agent Enrollment

In the dashboard: **AI → Agents → Enable Auto-Enrollment**

```yaml
# lattice config for agent enrollment
ai:
  agent_enrollment:
    enabled: true
    default_ttl: 4h
    auto_scope: read-only
    require_approval: true
```

## Step 3: Connect Claude Desktop

In Claude Desktop settings:

```json
{
  "mcpServers": {
    "lattice": {
      "command": "npx",
      "args": ["-y", "@lattice/mcp-server"],
      "env": {
        "LATTICE_SERVER_URL": "https://your-lattice-instance:8080",
        "LATTICE_API_KEY": "your-api-key"
      }
    }
  }
}
```

## Step 4: Agent Gets Auto-Enrolled

When Claude Desktop connects:
1. Lattice issues a temporary WireGuard keypair
2. The agent appears as a peer with TTL badge
3. All agent actions are audited with the agent's identity

## Next Steps

- [AI Capabilities Overview](/ai/)
- [Compliance Automation](/ai/compliance)
