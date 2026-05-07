# L4: Time-Travel Network Debugging

**状态**: 已确认，待实现
**设计决策**:
- 防抖策略: controller-runtime RequeueAfter 30s + StateHash 去重
- 快照粒度: 全量快照（每条 <10KB），通过 StateHash 跳过无变更周期
- AI Debug: SSE 流式

---

## 1. 数据模型

### NetworkSnapshot

**文件**: `internal/server/models/network_snapshot.go`

```go
type NetworkSnapshot struct {
    ID          string    `gorm:"primaryKey;type:varchar(36);not null"`
    WorkspaceID string    `gorm:"size:36;index"`
    Namespace   string    `gorm:"size:200"`
    CapturedAt  time.Time `gorm:"index"`
    TriggerType string    `gorm:"size:30;index"` // policy_change | peer_online | peer_offline | workflow_executed | manual | scheduled
    TriggerBy   string    `gorm:"size:100"`       // user ID or event source

    StateHash   string `gorm:"size:64;index"` // SHA256 of serialized CRD state, for change detection
    PeersJSON    string `gorm:"type:text"`     // JSON array of LatticePeer status
    PoliciesJSON string `gorm:"type:text"`     // JSON array of LatticePolicy status
    NetworksJSON string `gorm:"type:text"`     // JSON array of LatticeNetwork status
    PresenceJSON string `gorm:"type:text"`     // JSON map[peerID]online_status
}

func (NetworkSnapshot) TableName() string { return "t_network_snapshot" }
```

**索引策略**:
- `WorkspaceID` + `CapturedAt` 复合索引: 按 workspace + 时间范围查快照
- `StateHash`: 防抖去重

## 2. 存储层

### Repository 接口

**文件**: `internal/agent/store/store.go`

```go
type NetworkSnapshotRepository interface {
    Create(ctx context.Context, snap *models.NetworkSnapshot) error
    GetByID(ctx context.Context, id string) (*models.NetworkSnapshot, error)
    List(ctx context.Context, filter NetworkSnapshotFilter) ([]*models.NetworkSnapshot, int64, error)
    DeleteOlderThan(ctx context.Context, before time.Time) error
    GetLatestByWorkspace(ctx context.Context, workspaceID string) (*models.NetworkSnapshot, error)
    GetByWorkspaceAndTimeRange(ctx context.Context, workspaceID string, from, to time.Time) ([]*models.NetworkSnapshot, error)
}

type NetworkSnapshotFilter struct {
    WorkspaceID string
    TriggerType string
    From        time.Time
    To          time.Time
    Page        int
    PageSize    int
}
```

### GORM 实现

**文件**: `internal/db/gormstore/network_snapshot.go`

标准 CRUD + 按时间范围查询 + 过期删除。

## 3. 快照捕获控制器

**文件**: `internal/server/controller/snapshot_controller.go`

使用 controller-runtime 的 Reconciler 模式:

```
┌──────────────────────────────┐
│  SnapshotReconciler          │
│  RequeueAfter: 30s           │
│                              │
│  1. List all workspaces      │
│  2. For each workspace:      │
│     a. Get current CRD state │
│     b. Compute StateHash     │
│     c. Compare with last     │
│        snapshot's hash       │
│     d. Changed? → snapshot   │
│     e. Unchanged → skip      │
│                              │
│  3. Also watch CRD events:   │
│     Informer on Peer/Policy  │
│     → trigger immediate      │
│       reconcile              │
└──────────────────────────────┘
```

**快速响应路径**:
- Informer 监听 `LatticePeer` + `LatticePolicy` CRD 变更
- 变更时触发 `reconcile.Request{NamespacedName}` 立即 reconcile
- RequeueAfter 30s 兜底（防止事件丢失）

**手动快照**: PRE 端点 `POST /api/v1/workspaces/:id/snapshots` 直接调 `Capture()`。

## 4. HTTP API

**文件**: `internal/server/server/debug.go`

### 端点

```
# 列出快照（分页，支持时间范围）
GET    /api/v1/workspaces/:id/snapshots?from=&to=&page=&pageSize=

# 获取单条快照
GET    /api/v1/workspaces/:id/snapshots/:snapshotId

# 对比两个快照
GET    /api/v1/workspaces/:id/snapshots/diff?from=:id1&to=:id2

# 手动触发快照（Community: OK, Pro: 无额外限制）
POST   /api/v1/workspaces/:id/snapshots

# AI Debug 根因分析（Pro）
POST   /api/v1/ai/debug
```

### Diff 响应格式

```json
{
  "from": { "id": "snap-1", "capturedAt": "..." },
  "to":   { "id": "snap-2", "capturedAt": "..." },
  "peersAdded":     ["peer-3"],
  "peersRemoved":   ["peer-1"],
  "peersChanged":   ["peer-2"],
  "policiesAdded":  ["policy-allow-api"],
  "policiesRemoved":[],
  "policiesChanged":[],
  "connectivityChanges": [
    {"from": "frontend", "to": "api-gateway", "before": "ALLOW", "after": "DENY"}
  ]
}
```

### Debug Request/Response (SSE)

```
POST /api/v1/ai/debug
Content-Type: application/json

{
  "workspaceId": "ws-xxx",
  "question": "为什么 payment-service 连不上 postgres？",
  "timeRange": {
    "from": "2026-05-05T10:00:00Z",
    "to":   "2026-05-05T11:00:00Z"
  }
}

Response: text/event-stream
data: {"type":"token","content":"正在分析网络状态..."}
data: {"type":"token","content":"在 10:32:05，payment-service 的策略从 ALLOW 变更为 DENY"}
data: {"type":"done"}
```

AIService 中的 Debug 流程:
1. AI 识别需要查哪些快照 → 调用 `list_snapshots` 工具
2. AI 获取特定快照 → 调用 `get_snapshot` 工具
3. AI 对比两个快照 → 调用 `diff_snapshots` 工具
4. AI 综合判断给出根因分析

## 5. MCP 调试工具

在 `AIService.ListTools` 和 `ExecuteTool` 中新增（Pro 专属）：

| 工具 | 说明 |
|------|------|
| `list_snapshots` | 列出某 workspace 时间范围内的快照 |
| `get_snapshot` | 获取某时间点的完整网络状态 |
| `diff_snapshots` | 对比两个快照，返回结构化 diff |
| `check_connectivity_at` | 在指定快照状态下模拟连通性检查 |

## 6. 保留策略

```go
type SnapshotRetentionPolicy struct {
    MaxAge  time.Duration // community: 90天, pro: 1年
    CleanupInterval time.Duration // 默认 1 小时
}
```

后台 goroutine 定期清理过期快照，使用 `internal/server/auth/revocation.go` 的 `StartCleanup` 模式。

## 7. Pro 特性门控

| 功能 | Community | Pro |
|------|-----------|-----|
| 自动快照捕获 | ✅ | ✅ |
| 快照列表/查看/Diff | ✅ | ✅ |
| 手动触发快照 | ✅ | ✅ |
| AI Debug 根因分析 (SSE) | 402 | ✅ |
| Debug MCP 工具 | 402 | ✅ |
| 保留期限 | 90 天 | 1 年 |

## 8. 文件映射

| 动作 | 文件 |
|------|------|
| Create | `internal/server/models/network_snapshot.go` |
| Create | `internal/db/gormstore/network_snapshot.go` |
| Modify | `internal/agent/store/store.go` |
| Modify | `internal/db/gormstore/store.go` |
| Modify | `internal/db/gormstore/migrate.go` |
| Create | `internal/server/controller/snapshot_controller.go` |
| Create | `internal/server/server/debug.go` |
| Modify | `internal/server/server/server.go` |
| Modify | `internal/server/server/api.go` |
| Modify | `internal/server/service/ai.go` |

## 9. 实现顺序

1. **Snapshot 模型 + 存储** — model, store interface, gorm impl, migration
2. **快照捕获控制器** — controller-runtime reconciler with RequeueAfter
3. **HTTP API 端点** — 列表/查看/Diff/手动触发
4. **AI Debug 扩展** — AIService.Debug + SSE + MCP 工具
5. **保留策略** — 后台清理 + 过期检查
