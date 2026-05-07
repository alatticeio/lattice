---
title: Time-Travel Network Debugging
---

# Time-Travel Network Debugging (Pro)

Network state is ephemeral — when something went wrong 10 minutes ago, the evidence is often gone. Lattice's Time-Travel Debugging captures **automatic snapshots** of your entire network state at every meaningful event, letting you inspect, diff, and AI-debug any point in the past.

> **Pro feature.** Available in the Pro edition.

## Snapshot Model

Each snapshot captures the complete network state at a point in time:

| Field | Description |
|-------|-------------|
| `peers` | All LatticePeer resources (JSON) |
| `policies` | All LatticePolicy resources (JSON) |
| `networks` | All LatticeNetwork resources (JSON) |
| `presence` | Peer online/offline status map |
| `trigger_type` | What caused the snapshot |
| `captured_at` | Timestamp |

Individual snapshots are under 10 KB (semantic JSON, `managedFields` stripped).

## Trigger Events

| Event | Trigger Type |
|-------|-------------|
| LatticePolicy created/updated/deleted | `policy_change` |
| LatticePeer comes online | `peer_online` |
| LatticePeer goes offline | `peer_offline` |
| WorkflowService execution completes | `workflow_executed` |
| Manual API call | `manual` |
| Daily at 2 AM | `scheduled` |

Snapshot controller (`snapshot_controller.go`) watches CRD change events with debouncing.
- **Retention**: 90 days (Pro: 1 year)

## API Endpoints

```http
GET  /api/v1/workspaces/:id/snapshots
GET  /api/v1/workspaces/:id/snapshots/:snapshotId
GET  /api/v1/workspaces/:id/snapshots/diff?from=:id1&to=:id2
POST /api/v1/ai/debug
```

### AI Debug

The `POST /api/v1/ai/debug` endpoint accepts natural language questions about past network state (SSE streaming):

```json
{
  "workspace_id": "ws-prod",
  "question": "what caused the connectivity loss between api-server and db at 2:30pm?",
  "time_range": {
    "from": "2026-05-06T14:00:00Z",
    "to": "2026-05-06T15:00:00Z"
  }
}
```

The AI has access to snapshots in the time range, can diff them, and runs connectivity simulations against historical state to identify root causes.

## MCP Debug Tools

When connected via MCP, these additional tools are available:

| Tool | Description |
|------|-------------|
| `list_snapshots` | List snapshots within a time range |
| `get_snapshot` | Get complete state at a specific point in time |
| `diff_snapshots` | Compare two snapshots and show changes |
| `check_connectivity_at` | Simulate connectivity check against historical state |

## Use Cases

- **"The database became unreachable 20 minutes ago — what changed?"**
- **"Show me all policy changes between 2pm and 3pm yesterday."**
- **"Was there a peer that went offline right before the incident?"**
- **"Compare the network state before and after the last deployment."**
