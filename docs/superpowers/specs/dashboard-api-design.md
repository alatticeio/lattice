# Dashboard API Design

## 1. Problem Statement

Three bugs block real telemetry data from reaching the dashboard:

| # | Bug | Location | Fix |
|---|-----|----------|-----|
| 1 | Scrapers emit `network_id` but queries filter `workspace_id` | `service/monitor.go` | Use `network_id` throughout |
| 2 | `wireflow_workspace_tunnels` never emitted | `service/monitor.go` | Replace with `ceil(sum(wireflow_peer_status == 1) / 2)` |
| 3 | `wireflow_node_uptime_seconds` never emitted | `scraper_system.go` | Emit process uptime in `SystemScraper` |

Additionally, there is no workspace-scoped dashboard endpoint — all users see the same global view.

---

## 2. Metric Inventory

### Existing (emitted by agent scrapers)

| Metric | Labels | Scraper |
|--------|--------|---------|
| `wireflow_node_cpu_usage_percent` | peer_id, **network_id** | system |
| `wireflow_node_memory_bytes` | peer_id, **network_id** | system |
| `wireflow_node_goroutines` | peer_id, **network_id** | system |
| `wireflow_peer_status` | peer_id, **network_id**, remote_peer_id, endpoint | wireguard |
| `wireflow_peer_last_handshake_seconds` | peer_id, **network_id**, remote_peer_id | wireguard |
| `wireflow_peer_traffic_bytes_total` | peer_id, **network_id**, remote_peer_id, direction | wireguard |
| `wireflow_node_traffic_bytes_total` | peer_id, **network_id**, direction | wireguard |
| `wireflow_peering_traffic_bytes_total` | peer_id, local_network_id, remote_network_id, direction | wireguard |
| `wireflow_peer_latency_ms` | peer_id, **network_id**, remote_peer_id, remote_peer_ip | icmp |
| `wireflow_peer_packet_loss_percent` | peer_id, **network_id**, remote_peer_id | icmp |

### New (added in this change)

| Metric | Labels | Purpose |
|--------|--------|---------|
| `wireflow_node_uptime_seconds` | peer_id, **network_id** | Count online nodes; gauge staleness |

**Label convention**: `network_id` = workspace `Namespace` field (e.g. `wf-abc123`). All PromQL filters must use `network_id`, not `workspace_id`.

---

## 3. PromQL Reference

### Workspace Dashboard Queries (namespace = `network_id` value)

#### Stat Cards

| Panel | PromQL |
|-------|--------|
| 在线节点数 | `count(last_over_time(wireflow_node_uptime_seconds{network_id="$ns"}[5m]))` |
| 实时吞吐 TX (Mbps) | `sum(irate(wireflow_node_traffic_bytes_total{network_id="$ns",direction="tx"}[2m])) * 8 / 1e6` |
| 平均延迟 (ms) | `avg(wireflow_peer_latency_ms{network_id="$ns"})` |
| 丢包率 (%) | `avg(wireflow_peer_packet_loss_percent{network_id="$ns"})` |

#### Charts

| Panel | PromQL | Mode |
|-------|--------|------|
| 吞吐趋势 TX | `sum(irate(wireflow_node_traffic_bytes_total{network_id="$ns",direction="tx"}[5m])) * 8 / 1e6` | range, 1h, step=2m |
| 吞吐趋势 RX | `sum(irate(wireflow_node_traffic_bytes_total{network_id="$ns",direction="rx"}[5m])) * 8 / 1e6` | range, 1h, step=2m |
| CPU per node | `last_over_time(wireflow_node_cpu_usage_percent{network_id="$ns"}[5m])` | instant |
| Memory per node | `last_over_time(wireflow_node_memory_bytes{network_id="$ns"}[5m])` | instant |

#### Tables

| Panel | PromQL |
|-------|--------|
| Top nodes (24h traffic) | `topk(10, sum by (peer_id)(increase(wireflow_node_traffic_bytes_total{network_id="$ns"}[24h])))` |
| Node online status | `last_over_time(wireflow_peer_status{network_id="$ns"}[5m])` |

---

## 4. API Contract

### Global Dashboard (platform_admin)
```
GET /api/v1/dashboard/overview
Authorization: Bearer <token>
```
Returns `DashboardResponse` — aggregated across all workspaces.

### Workspace Dashboard (workspace member)
```
GET /api/v1/workspaces/:id/dashboard
Authorization: Bearer <token>
```
Returns `WorkspaceDashboardResponse`:

```json
{
  "stat_cards": [
    { "label": "在线节点", "value": "12", "unit": "台",  "trend": "stable", "trend_pct": "", "color": "text-emerald-500" },
    { "label": "实时吞吐", "value": "234.5", "unit": "Mbps", "trend": "up",   "trend_pct": "+5%", "color": "text-blue-500" },
    { "label": "平均延迟", "value": "18.3",  "unit": "ms",   "trend": "stable","trend_pct": "",   "color": "text-amber-500" },
    { "label": "丢包率",   "value": "0.12",  "unit": "%",    "trend": "stable","trend_pct": "",   "color": "text-emerald-500" }
  ],
  "throughput_trend": {
    "timestamps": ["10:00", "10:02", "..."],
    "tx_data": [120.1, 131.4, "..."],
    "rx_data": [88.2,  91.0,  "..."]
  },
  "node_cpu": [
    { "peer_id": "abc", "name": "node-01", "cpu": 72.4, "memory_mb": 512.0 }
  ],
  "top_nodes": [
    { "id": "abc", "name": "node-01", "endpoint": "1.2.3.4:51820",
      "total_tx": 10485760, "total_rx": 5242880, "online": true, "cpu": 72.4 }
  ]
}
```

---

## 5. Architecture / Data Flow

```
Agent (Pro)
  └─ SystemScraper    → wireflow_node_cpu_usage_percent
                      → wireflow_node_memory_bytes
                      → wireflow_node_uptime_seconds   ← NEW
  └─ WireGuardScraper → wireflow_peer_status
                      → wireflow_node_traffic_bytes_total
                      → wireflow_peer_traffic_bytes_total
                      → ...
  └─ ICMPScraper      → wireflow_peer_latency_ms
                      → wireflow_peer_packet_loss_percent
        │
        │ Remote Write (protobuf+snappy)
        ▼
  VictoriaMetrics
        │
        │ PromQL HTTP API
        ▼
  service/monitor.go
    GetWorkspaceDashboard(namespace)   ← NEW (7 parallel queries)
    GetGlobalDashboard()               (fixed: network_id labels)
        │
        ▼
  controller/monitor.go
    GetWorkspaceDashboard(wsID)        ← translates wsID → ws.Namespace
        │
        ▼
  server/dashboard.go
    GET /api/v1/workspaces/:id/dashboard   ← NEW
    GET /api/v1/dashboard/overview         (existing)
        │
        ▼
  frontend
    api/dashboard.ts          getWorkspaceDashboard(wsID)  ← NEW
    stores/useDashboard.ts    fetchWorkspace() + wsData state  ← NEW
    pages/dashboard/index.vue auto-switch workspace/global mode  ← updated
```

---

## 6. Implementation Checklist

### Backend
- [x] `internal/telemetry/scraper_system.go` — add `wireflow_node_uptime_seconds`
- [x] `management/models/dashboard.go` — add `WorkspaceDashboardResponse`, `WorkspaceStatCard`, `NodeCPUItem`
- [x] `management/service/monitor.go` — fix labels, fix tunnels query, add `GetWorkspaceDashboard`
- [x] `management/controller/monitor.go` — add store, add `GetWorkspaceDashboard`, translate wsID→namespace
- [x] `management/server/server.go` — pass store to `NewMonitorController`
- [x] `management/server/dashboard.go` — add workspace dashboard route+handler

### Frontend
- [x] `fronted/src/api/dashboard.ts` — add `getWorkspaceDashboard`
- [x] `fronted/src/types/monitor.ts` — add workspace dashboard types
- [x] `fronted/src/stores/useDashboard.ts` — add workspace state + `fetchWorkspace` action
- [x] `fronted/src/pages/dashboard/index.vue` — auto-switch workspace/global mode
