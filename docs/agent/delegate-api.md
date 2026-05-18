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
