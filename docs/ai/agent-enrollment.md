---
title: Zero-Trust AI Agent Networking
---

# Zero-Trust AI Agent Networking

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
