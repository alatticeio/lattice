---
title: Network Intent Engine
---

# Network Intent Engine (Pro)

Describe network changes in natural language. The Intent Engine converts your intent into a structured CRD change plan, complete with risk assessment and human-readable diff — without touching YAML.

> **Pro feature.** The Intent Engine is available in the Pro edition. Community edition returns a 402 Payment Required.

## Two-Stage LLM Pipeline

The Intent Engine uses a two-stage LLM pipeline to separate precision from readability:

**Stage 1 — Structured Intent Extraction** (strong JSON schema constraint)
- Input: user's natural language + current network state
- Output: `{ changes: CRDChange[], risk: string, reasoning: string }`

**Stage 2 — Human-Readable Diff Generation**
- Output: Markdown summary for UI display and approval emails

This separation exists because precision (Stage 1) and readability (Stage 2) require different prompting strategies that are difficult to achieve in a single call.

## API

### Plan — Preview Changes

```http
POST /api/v1/ai/intent/plan
```

Produces a change plan without executing anything:

```json
{
  "workspace_id": "ws-prod",
  "intent": "allow api-server to connect to redis on port 6379",
  "dry_run": true
}
```

Response:

```json
{
  "plan_id": "plan-abc123",
  "summary": "Create a new policy allowing api-server egress to redis on port 6379",
  "risk_level": "low",
  "changes": [
    {
      "action": "create",
      "resource": "LatticePolicy",
      "before": "",
      "after": "apiVersion: alattice.io/v1alpha1\nkind: LatticePolicy\n..."
    }
  ],
  "expires_at": "2026-05-06T12:00:00Z"
}
```

### Apply — Execute Approved Plan

```http
POST /api/v1/ai/intent/apply
```

```json
{
  "plan_id": "plan-abc123",
  "approved_by": "user@company.com"
}
```

Returns a `workflow_id` that enters the WorkflowService approval pipeline.

## MCP Tools

When connected via MCP, the Intent Engine exposes two additional tools:

| Tool | Description |
|------|-------------|
| `plan_network_change(intent)` | Preview a network change from natural language |
| `apply_network_change(plan_id)` | Execute an approved plan (enters approval workflow) |

## Architecture

The Intent Engine follows the same architectural patterns as all Lattice AI features:

- **CRD as Source of Truth** — all proposed changes are expressed as CRD YAML
- **Human-in-the-loop** — changes go through WorkflowService approval
- **Pro gated** — community edition stubs return a payment required error
