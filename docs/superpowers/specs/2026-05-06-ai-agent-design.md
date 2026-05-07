# AI Agent Design

> Status: Approved | 2026-05-06

## Motivation

Lattice is an enterprise WireGuard overlay network. Traditional network management requires CLI expertise, YAML authoring, and deep networking knowledge. The AI Agent lowers this barrier by:

- **ChatOps via MCP**: Operators interact with the network through natural language in their existing tools (Claude Desktop, Cursor, etc.)
- **Intent-Driven Operations**: Describe what you want ("allow frontend to reach API"), not how (no YAML authoring needed)
- **Time-Travel Debugging**: AI-assisted root cause analysis across historical network state snapshots
- **Compliance Auditing**: Automated security posture scoring with LLM-enriched remediation suggestions

This positions Lattice as the first WireGuard overlay with deeply integrated AI-native operations — a differentiator against Tailscale, Netbird, and ZeroTier.

## Architecture

```
                    ┌──────────────────────────────────────────┐
                    │              latticed / manager           │
                    │                                          │
  Claude Desktop    │  ┌──────────┐  ┌──────────────┐         │
  Cursor IDE ───────┼─>│ lattice- │─>│ aiService    │─> LLM   │──> Anthropic
  (MCP stdio)       │  │ mcp      │  │ (agent loop) │         │    / DeepSeek
                    │  └──────────┘  └──────┬───────┘         │
  Web UI ───────────┼──────────────────────┘                  │
  (Vue 3 SPA)       │  ┌──────────────────────────────────┐   │
                    │  │ Gin HTTP /api/v1/ai/*             │   │
                    │  └──────────────────────────────────┘   │
                    │             │            │               │
                    │  ┌──────────▼──┐ ┌───────▼──────────┐   │
                    │  │ Snapshot    │ │ AgentTTL         │   │
                    │  │ Controller  │ │ Controller       │   │
                    │  │ (watch CRDs)│ │ (watch LatticePeer)│  │
                    │  └─────────────┘ └──────────────────┘   │
                    └──────────────────────────────────────────┘
```

### Layered Design

| Layer | Component | Responsibility |
|-------|-----------|---------------|
| Entry | `lattice-mcp` (stdio) | MCP protocol gateway, proxies tools → HTTP API |
| Entry | Web UI (Vue 3) | 4+1 pages: Chat, Intent, Debug, Compliance, Tools |
| Transport | Gin HTTP `/api/v1/ai/*` | SSE streaming, JSON-RPC proxy, auth middleware |
| Core | `aiService` | Agentic loop, tool dispatch, audit engine |
| LLM | `llm.Client` | Provider-agnostic abstraction (Anthropic / OpenAI-compat) |
| State | K8s CRDs | LatticePeer, LatticePolicy, LatticeNetwork — source of truth |
| State | GORM store | IntentPlan (10min TTL), NetworkSnapshot (90d/1y retention) |
| State | NATS | Peer online presence / heartbeats |

### AI Features Feature Gate

When the server starts with `ai.enabled=false` (no `api-key` configured), the `aiRouter()` registers stub handlers returning HTTP 503. All AI endpoints are behind `AuthMiddleware`.

---

## Module 1: MCP Server & ChatOps

### MCP Server (`lattice-mcp`)

A standalone binary that speaks the [Model Context Protocol](https://modelcontextprotocol.io/) over stdin/stdout (JSON-RPC 2.0). Operators register it as an MCP tool provider in Claude Desktop, Cursor, or any MCP-compatible client.

```
lattice-mcp --server-url http://localhost:8080 --token <auth-token> --workspace <id>
```

**Protocol flow:**

| Method | Direction | Purpose |
|--------|-----------|---------|
| `initialize` | Client → Server | Capability negotiation (tools only) |
| `initialized` | Client → Server | Handshake complete notification |
| `tools/list` | Client → Server | Returns available tools (proxied from latticed) |
| `tools/call` | Client → Server | Invokes a tool with workspace-scoped parameters |

**Tool dispatch** (14 tools available):

*Read tools (no side effects):*
| Tool | Purpose |
|------|---------|
| `list_peers` | List all peers in workspace |
| `list_policies` | List all policies in workspace |
| `list_networks` | List all networks in workspace |
| `check_connectivity` | Check peer-to-peer connectivity |
| `list_snapshots` | Browse network state snapshots (Pro) |
| `get_snapshot` | Get a specific snapshot (Pro) |
| `diff_snapshots` | Compare two snapshots (Pro) |
| `check_connectivity_at` | Check connectivity at a past time (Pro) |
| `plan_network_change` | Generate intent plan (Pro) |

*Write tools (routed through workflow approval):*
| Tool | Purpose | Auto-Approve |
|------|---------|-------------|
| `create_policy` | Create a network policy | No (needs approval) |
| `delete_peer` | Remove a peer | No |
| `delete_policy` | Remove a policy | No |
| `create_peer` | Create a new peer | No |
| `apply_network_change` | Execute an intent plan | No |

### AI Chat (Web UI)

The `/ai` chat page provides an in-app alternative to the MCP gateway. It uses SSE streaming (`POST /api/v1/ai/chat`) with the same agentic loop and tool set. Each event is a typed SSE message:

| Event type | Content |
|------------|---------|
| `token` | Streaming LLM text chunk |
| `tool_use` | `{ tool, input }` — AI is invoking a tool |
| `error` | Error message |
| `done` | Stream complete |

### Agentic Loop

```
1. Resolve workspace → load peers, policies, networks from K8s CRDs
2. Build system prompt with current network state
3. Loop (max 5 tool calls):
   a. Send messages + tools → LLM
   b. If no tool call → stream final answer, exit
   c. If tool calls → execute each, stream tool_use events
   d. Append tool results to message history
4. If tool budget exhausted → final LLM call for summary
```

---

## Module 2: Agent Enrollment

AI agents are ephemeral, managed peers that participate in the network for a bounded time.

### Enroll Flow

```
POST /api/v1/agent-enroll
{
  "workspaceId": "...",
  "name": "mcp-agent-001",
  "agentType": "coordinator",
  "ttl": "1h",
  "policyPreset": "sandboxed"
}
```

**Policy presets:**
| Preset | Access |
|--------|--------|
| `sandboxed` | Minimal — can only reach the control plane |
| `coordinator` | Can reach all peers, useful for orchestration |
| `isolated` | No network access, API-only |

### TTL Lifecycle

An `AgentTTLReconciler` watches `LatticePeer` resources labeled `lattice.io/agent-managed=true`. Each peer has an annotation `lattice.io/agent-expires-at` with an RFC 3339 timestamp. The reconciler:
1. Watches for `LatticePeer` events with the agent label
2. On each event, checks if the TTL has expired
3. If expired, deletes the peer CRD
4. Requeues at the exact expiry time for precise cleanup

### Revoke

```
DELETE /api/v1/agent-enroll/:peerName
```

Immediately removes the agent peer without waiting for TTL expiry.

---

## Module 3: Intent Engine (Pro)

### Overview

Natural language → CRD change plan → human approval → execution.

The Intent Engine is a two-phase LLM pipeline that translates natural language intents into executable Kubernetes CRD changes, with risk assessment and human-readable summaries.

### Phase 1: Plan Generation

```
POST /api/v1/ai/intent/plan
{
  "workspaceId": "...",
  "intent": "allow the frontend service to connect to the API gateway on port 443",
  "dryRun": true
}
```

1. Snapshot current K8s state (peers, policies) from the workspace
2. Call LLM with structured prompt to extract JSON: `{ changes: CRDChange[], riskLevel: string, reasoning: string }`
3. Call LLM again for a human-readable Markdown summary
4. Persist as `IntentPlan` with 10-minute TTL

**CRDChange type:**
```go
type CRDChange struct {
    Action   string // "create", "delete", "update"
    Resource string // policy name, peer name, etc.
}
```

**Risk levels:** `low` (emerald), `medium` (amber), `high` (rose)

### Phase 2: Apply

```
POST /api/v1/ai/intent/apply
{ "planId": "..." }
```

1. Load the `IntentPlan` from DB
2. Verify TTL has not expired
3. Submit changes through workflow approval
4. Return workflow IDs for tracking

### Community Edition

The community stub returns HTTP 402 (`Payment Required`) with a message directing to upgrade to Pro.

---

## Module 4: Time-Travel Debugging (Pro)

### Overview

A 4-layer system for AI-assisted root cause analysis across historical network state:

```
Layer 1: Snapshot Capture (controller-runtime)
    ↓
Layer 2: GORM-backed Snapshot Store
    ↓
Layer 3: AI Debug Tools (agentic loop with debug-specific tools)
    ↓
Layer 4: Web UI (snapshot timeline + AI debug chat)
```

### Layer 1: Snapshot Capture

The `SnapshotController` watches `LatticePolicy` CRDs and triggers on every change:
- **Debounce**: 1-second window to batch rapid changes
- **Dedup**: SHA256 `StateHash` on the JSON payload — skip if unchanged
- **Schedule**: `RequeueAfter: 30s` for periodic capture even without CRD events
- **Manual trigger**: `POST /api/v1/workspaces/:id/snapshots`

Each `NetworkSnapshot` captures:
- Peers: name, labels, overlay IP
- Policies: name, action, network, selector
- Networks: name, phase, CIDR
- Presence: NATS heartbeat status

### Layer 2: Storage

| Field | Type | Purpose |
|-------|------|---------|
| `WorkspaceID` | string | Scope to workspace |
| `CapturedAt` | time | When the snapshot was taken |
| `TriggerType` | enum | `policy_change`, `peer_online`, `peer_offline`, `workflow_executed`, `manual`, `scheduled` |
| `StateHash` | string | SHA256 dedup key |
| `Peers` | JSON | Array of peer states |
| `Policies` | JSON | Array of policy states |
| `Networks` | JSON | Array of network states |
| `Presence` | JSON | Online/offline status |

**Retention**: Community 90 days, Pro 1 year. Cleaned by background goroutine.

### Layer 3: Debug Tools

Four Pro-only tools available to the AI agent:

| Tool | Purpose |
|------|---------|
| `list_snapshots` | List snapshots with time-range filtering |
| `get_snapshot` | Get full state at a point in time |
| `diff_snapshots` | Structured diff (added/removed/changed) between two snapshots |
| `check_connectivity_at` | Check if two peers could communicate at a past time |

The `Debug()` method implements a separate agentic loop with a debug-specific system prompt and restricted tool set (only these 4 tools).

### Layer 4: Debug UI

Two-panel layout:
- **Left (Snapshot Timeline)**: Chronological list of snapshots with trigger type badges. Select any two for diff comparison.
- **Right (AI Debug Chat)**: Ask natural language questions ("why did peer X lose connectivity yesterday?") with SSE streaming responses. Diff results displayed inline.

### API Endpoints

```
GET    /api/v1/workspaces/:id/snapshots          List (paginated, ?from=&to=&triggerType=)
GET    /api/v1/workspaces/:id/snapshots/:sid     Get single
GET    /api/v1/workspaces/:id/snapshots/diff     Compare (?from=&to=)
POST   /api/v1/workspaces/:id/snapshots          Manual trigger
POST   /api/v1/ai/debug                          AI debug (SSE streaming)
```

---

## Module 5: Compliance Auditing (Pro)

### Overview

Automated network security posture scoring with LLM-enriched remediation suggestions.

### Audit Rules (Hard-coded)

| Rule | Severity | Detection |
|------|----------|-----------|
| `allow-all-detected` | High | Policy with `ALLOW` action + no ingress/egress constraints + no peer selectors |
| `no-policies` | Medium | Workspace has peers but zero policies defined |
| `unused-peer` | Low | Peer with empty or "offline" NATS heartbeat status |

### Scoring

```
Score starts at 100
-15 per high finding
-8 per medium finding
-3 per low finding
Minimum: 0
```

Score bands:
- >= 80: Good (emerald)
- >= 60: Fair (amber)
- < 60: Poor (rose)

### LLM Enrichment

When findings exist, each one is enriched by an LLM call (`enrichFindingsWithLLM()`) that generates:
- Human-readable Chinese description of the issue
- Actionable remediation suggestion

### API

```
GET /api/v1/ai/audit?workspaceId=<id>
```

Response:
```json
{
  "score": 70,
  "generatedAt": "2026-05-06T10:00:00Z",
  "findings": [
    {
      "severity": "high",
      "rule": "allow-all-detected",
      "resource": "default-allow-all",
      "description": "策略 default-allow-all 允许所有流量，存在安全风险",
      "suggestion": "建议添加具体的 ingress/egress 规则限制流量范围"
    }
  ]
}
```

### Compliance UI

Single-page dashboard:
- **Score card**: Large numeric score with color-coded badge
- **Severity summary**: Three count cards (high/medium/low)
- **Searchable findings table**: Columns for severity badge, rule name, resource, description, suggestion
- **Refresh**: Manual re-trigger
- **Error handling**: HTTP 402 → "Pro feature required"; network errors → retry button

---

## Technology Stack

| Component | Technology |
|-----------|-----------|
| AI Chat UI | Vue 3 + Tailwind 4 + SSE streaming |
| Intent Engine | LLM 2-phase pipeline + GORM + 10min TTL |
| Debug Tool | LLM agentic loop + GORM + controller-runtime |
| Compliance | LLM enrichment + hard-coded rules + scoring |
| MCP Server | JSON-RPC 2.0 over stdio |
| LLM Backend | Provider-agnostic: Anthropic (native) + OpenAI-compat (DeepSeek) |
| SSE Adapter | Gin `text/event-stream` with typed events |

## File Layout

```
cmd/lattice-mcp/main.go                     MCP binary entry point
internal/mcp/server.go                      MCP stdio server
internal/mcp/protocol.go                    MCP protocol types
internal/mcp/server_test.go                 MCP server tests
internal/server/server/ai.go                AI HTTP route handlers + SSE writer
internal/server/server/agent.go             Agent enroll/revoke handlers
internal/server/service/ai.go               Core aiService (agentic loop, tools, audit)
internal/server/service/intent.go           IntentService interface + types
internal/server/service/intent_pro.go       Pro IntentService implementation
internal/server/service/intent_community.go Community stub (402)
internal/server/service/agent_enroll.go     Agent enrollment service
internal/server/llm/client.go               LLM client interface
internal/server/llm/anthropic.go            Anthropic provider
internal/server/llm/openai_compat.go        OpenAI-compatible provider
internal/server/llm/factory.go              LLM client factory
internal/server/models/intent_plan.go       IntentPlan DB model
internal/server/models/network_snapshot.go  NetworkSnapshot DB model
internal/db/gormstore/intent_plan.go        IntentPlan repository
internal/db/gormstore/network_snapshot.go   NetworkSnapshot repository
internal/server/controller/snapshot_controller.go   Snapshot capture
internal/server/controller/agent_ttl_controller.go  TTL-based peer cleanup
fronted/src/pages/ai/index.vue              AI Chat page
fronted/src/pages/ai/intent.vue             Intent Engine page
fronted/src/pages/ai/debug.vue              Time-Travel Debug page
fronted/src/pages/ai/compliance.vue          Compliance Audit page
fronted/src/pages/ai/tools.vue              MCP Tool Browser page
fronted/src/api/ai.ts                       AI chat + compliance API client
fronted/src/api/intent.ts                   Intent plan API client
fronted/src/api/debug.ts                    Debug streaming API client
fronted/src/api/tools.ts                    MCP tools API client
fronted/src/api/snapshot.ts                 Snapshot API client
```
