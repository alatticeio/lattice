# Agent Platform Integrated Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 gVisor 沙箱移入 Community、添加 MCP 工具调用可观测性（tool_spans）、实现 Sub-agent Delegate API、并将 gVisor AuditWriter 改为 NATS 上报控制面（PRO）。

**Architecture:** 在现有 agent-isolation 服务基础上差量补齐：gVisor 去掉 build tag 移入 community；ExecuteTool 包一层 traceID + writeToolSpan；AgentRegistrationService 新增 DelegateToken；PRO fileAuditWriter 替换为 natsAuditWriter。

**Tech Stack:** Go 1.25, GORM + SQLite/MySQL, Gin, NATS, gVisor (lattice-shim), wireguard-go, Ginkgo v2

---

## 文件结构

### 新增文件

| 文件 | 说明 |
|------|------|
| `cmd/lattice-agent-sandbox/cmd/wg_config.go` | ✅ 已存在 — formatWGConfig 共用函数 |
| `internal/server/models/tool_span.go` | Sprint 2 — ToolSpan GORM 模型 |
| `internal/db/gormstore/tool_span.go` | Sprint 2 — ToolSpanRepo 实现 |
| `internal/server/models/flow_event.go` | Sprint 4 — FlowEvent GORM 模型 |
| `internal/db/gormstore/flow_event.go` | Sprint 4 — FlowEventRepo 实现 |
| `internal/server/controller/audit_consumer.go` | Sprint 4 (PRO) — NATS audit 订阅器 |

### 修改文件

| 文件 | Sprint | 说明 |
|------|--------|------|
| `cmd/lattice-agent-sandbox/cmd/start_sandbox_community.go` | ✅ S1 | 实现真正的 gVisor 沙箱（已完成） |
| `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` | ✅ S1 / S4 | S1: 提取 formatWGConfig；S4: fileAuditWriter → natsAuditWriter |
| `internal/agent/gvisor/sandbox.go` | ✅ S1 | 去掉 `//go:build pro`（已完成） |
| `internal/agent/gvisor/shim_adapter.go` | ✅ S1 | 去掉 `//go:build pro`（已完成） |
| `internal/agent/gvisor/wg_bridge.go` | ✅ S1 | 去掉 `//go:build pro`（已完成） |
| `internal/agent/gvisor/wg_device.go` | ✅ S1 | 去掉 `//go:build pro`（已完成） |
| `internal/server/models/agent_claims.go` | S3 | 新增 ParentAgentID 字段 |
| `internal/server/models/agent_enrollment.go` | S3 | 新增 ParentAgentID 字段 |
| `api/v1alpha1/agent_identity_types.go` | S3 | 新增 ParentRef、SpawnableRoles 字段 |
| `internal/agent/store/store.go` | S2, S3, S4 | 新增 ToolSpans()、FlowEvents() 到 Store 接口 |
| `internal/db/gormstore/migrate.go` | S2, S4 | 注册新表 |
| `internal/db/gormstore/store.go` | S2, S4 | 注册新 repo |
| `internal/server/service/ai.go` | S2 | 包装 ExecuteTool，加 traceID + writeToolSpan |
| `internal/server/service/agent_registration.go` | S3 | 新增 DelegateToken() 方法，更新 RegisterAgent 读 parentAgentID |
| `internal/server/server/agent_isolation_router.go` | S2, S3 | 新增 /delegate、/audit/traces 路由 |

---

## Sprint 1: gVisor 移入 Community ✅ 已完成

本 Sprint 在当前会话中已全部实现：

- `internal/agent/gvisor/community_stub.go` — 已删除
- `internal/agent/gvisor/sandbox.go` 等 4 个文件 — 已去掉 `//go:build pro` 标签
- `cmd/lattice-agent-sandbox/cmd/start_sandbox_community.go` — 已重写为真正的 gVisor 沙箱
- `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` — 已提取 formatWGConfig 到共用文件
- `cmd/lattice-agent-sandbox/cmd/wg_config.go` — 已新增

验证命令（确认编译通过）：

```bash
go build ./internal/agent/gvisor/ 2>&1           # 期望: 无输出
go build -tags pro ./internal/agent/gvisor/ 2>&1 # 期望: 无输出
go build ./cmd/lattice-agent-sandbox/... 2>&1    # 期望: 无输出
go build -tags pro ./cmd/lattice-agent-sandbox/... 2>&1 # 期望: 无输出
```

---

## Sprint 2: MCP Trace Middleware

**验收标准：** 每次 ExecuteTool 调用后，`GET /api/v1/audit/traces?agentId=xxx` 能查到记录。

### Task 2.1: ToolSpan 模型 + 迁移

**Files:**
- Create: `internal/server/models/tool_span.go`
- Modify: `internal/db/gormstore/migrate.go`
- Modify: `internal/agent/store/store.go`
- Modify: `internal/db/gormstore/store.go`
- Create: `internal/db/gormstore/tool_span.go`
- Test: `internal/db/gormstore/tool_span_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/db/gormstore/tool_span_test.go
package gormstore_test

import (
    "context"
    "testing"
    "time"

    "github.com/alatticeio/lattice/internal/db/gormstore"
    "github.com/alatticeio/lattice/internal/server/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupToolSpanDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatalf("open db: %v", err)
    }
    if err := db.AutoMigrate(&models.ToolSpan{}); err != nil {
        t.Fatalf("migrate: %v", err)
    }
    return db
}

func TestToolSpan_WriteAndGet(t *testing.T) {
    db := setupToolSpanDB(t)
    repo := gormstore.NewToolSpanRepo(db)
    ctx := context.Background()

    span := &models.ToolSpan{
        TraceID:    "trace-001",
        AgentID:    "agent-a",
        ParentID:   "",
        Namespace:  "wf-test",
        Tool:       "list_peers",
        Status:     "ok",
        DurationMs: 42,
        StartedAt:  time.Now().UTC().Truncate(time.Second),
    }
    if err := repo.Write(ctx, span); err != nil {
        t.Fatalf("Write: %v", err)
    }
    got, err := repo.Get(ctx, "trace-001")
    if err != nil {
        t.Fatalf("Get: %v", err)
    }
    if got.Tool != "list_peers" {
        t.Errorf("tool: got %q, want %q", got.Tool, "list_peers")
    }
}

func TestToolSpan_List(t *testing.T) {
    db := setupToolSpanDB(t)
    repo := gormstore.NewToolSpanRepo(db)
    ctx := context.Background()

    for i := range 3 {
        _ = repo.Write(ctx, &models.ToolSpan{
            TraceID:   fmt.Sprintf("t%d", i),
            AgentID:   "agent-a",
            Tool:      "list_peers",
            Status:    "ok",
            StartedAt: time.Now().UTC(),
        })
    }
    spans, err := repo.List(ctx, "agent-a", time.Time{}, time.Now().Add(time.Hour), 10)
    if err != nil {
        t.Fatalf("List: %v", err)
    }
    if len(spans) != 3 {
        t.Errorf("expected 3 spans, got %d", len(spans))
    }
}
```

- [ ] **Step 2: 运行，确认失败**

```bash
go test ./internal/db/gormstore/... -run TestToolSpan -v 2>&1
```

期望：FAIL — `gormstore.NewToolSpanRepo` 未定义

- [ ] **Step 3: 创建 ToolSpan 模型**

```go
// internal/server/models/tool_span.go
package models

import "time"

// ToolSpan records a single MCP tool call for observability.
type ToolSpan struct {
    Model
    TraceID    string    `gorm:"uniqueIndex;size:36" json:"traceId"`
    AgentID    string    `gorm:"index;size:128"     json:"agentId"`
    ParentID   string    `gorm:"index;size:128"     json:"parentId,omitempty"`
    Namespace  string    `gorm:"size:128"           json:"namespace"`
    Tool       string    `gorm:"size:128"           json:"tool"`
    Status     string    `gorm:"size:32"            json:"status"` // ok | error | blocked
    ErrorMsg   string    `gorm:"type:text"          json:"errorMsg,omitempty"`
    DurationMs int64     `json:"durationMs"`
    StartedAt  time.Time `gorm:"index"              json:"startedAt"`
}

func (ToolSpan) TableName() string { return "la_tool_spans" }
```

- [ ] **Step 4: 创建 ToolSpanRepo 实现**

```go
// internal/db/gormstore/tool_span.go
package gormstore

import (
    "context"
    "time"

    "github.com/alatticeio/lattice/internal/server/models"
    "gorm.io/gorm"
)

type toolSpanRepo struct{ db *gorm.DB }

func NewToolSpanRepo(db *gorm.DB) *toolSpanRepo {
    return &toolSpanRepo{db: db}
}

func (r *toolSpanRepo) Write(ctx context.Context, span *models.ToolSpan) error {
    return r.db.WithContext(ctx).Create(span).Error
}

func (r *toolSpanRepo) Get(ctx context.Context, traceID string) (*models.ToolSpan, error) {
    var s models.ToolSpan
    if err := r.db.WithContext(ctx).Where("trace_id = ?", traceID).First(&s).Error; err != nil {
        return nil, err
    }
    return &s, nil
}

func (r *toolSpanRepo) List(ctx context.Context, agentID string, from, to time.Time, limit int) ([]*models.ToolSpan, error) {
    var spans []*models.ToolSpan
    q := r.db.WithContext(ctx).Order("started_at desc")
    if agentID != "" {
        q = q.Where("agent_id = ?", agentID)
    }
    if !from.IsZero() {
        q = q.Where("started_at >= ?", from)
    }
    if !to.IsZero() {
        q = q.Where("started_at <= ?", to)
    }
    if limit > 0 {
        q = q.Limit(limit)
    }
    return spans, q.Find(&spans).Error
}
```

- [ ] **Step 5: 在 store.go 接口添加 ToolSpans()**

在 `internal/agent/store/store.go` 中：

```go
// 在 Store interface 中添加（RefreshTokens() 之后）：
ToolSpans() ToolSpanRepository

// 在文件末尾添加 ToolSpanRepository 接口：

// ToolSpanRepository records and queries MCP tool call spans.
type ToolSpanRepository interface {
    Write(ctx context.Context, span *models.ToolSpan) error
    Get(ctx context.Context, traceID string) (*models.ToolSpan, error)
    List(ctx context.Context, agentID string, from, to time.Time, limit int) ([]*models.ToolSpan, error)
}
```

- [ ] **Step 6: 在 gormstore/migrate.go 注册新表**

在 `internal/db/gormstore/migrate.go` 的 `AutoMigrate(...)` 调用中，在 `&models.RefreshToken{},` 之后添加：

```go
&models.ToolSpan{},
```

- [ ] **Step 7: 在 gormstore/store.go 注册 repo**

在 `GormStore` struct 中添加字段：
```go
toolSpans store.ToolSpanRepository
```

在 `newStore()` 中添加：
```go
toolSpans: NewToolSpanRepo(db),
```

添加访问方法：
```go
func (s *GormStore) ToolSpans() store.ToolSpanRepository { return s.toolSpans }
```

- [ ] **Step 8: 运行测试，确认通过**

```bash
go test ./internal/db/gormstore/... -run TestToolSpan -v 2>&1
```

期望：PASS

- [ ] **Step 9: 确认整体编译通过**

```bash
go build ./... 2>&1
```

期望：无输出（编译成功）

- [ ] **Step 10: Commit**

```bash
git add internal/server/models/tool_span.go \
        internal/db/gormstore/tool_span.go \
        internal/db/gormstore/tool_span_test.go \
        internal/db/gormstore/migrate.go \
        internal/db/gormstore/store.go \
        internal/agent/store/store.go
git commit -s -m "feat(trace): add ToolSpan model and GORM repository"
```

---

### Task 2.2: 在 ExecuteTool 注入 traceID

**Files:**
- Modify: `internal/server/service/ai.go`
- Test: `internal/server/service/ai_trace_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/server/service/ai_trace_test.go
package service_test

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/alatticeio/lattice/internal/server/models"
    "github.com/alatticeio/lattice/internal/server/service"
    . "github.com/onsi/gomega"
)

// fakeToolSpanRepo记录所有Write调用。
type fakeToolSpanRepo struct {
    spans []*models.ToolSpan
}

func (r *fakeToolSpanRepo) Write(_ context.Context, s *models.ToolSpan) error {
    r.spans = append(r.spans, s)
    return nil
}
func (r *fakeToolSpanRepo) Get(_ context.Context, _ string) (*models.ToolSpan, error) {
    return nil, nil
}
func (r *fakeToolSpanRepo) List(_ context.Context, _ string, _, _ time.Time, _ int) ([]*models.ToolSpan, error) {
    return nil, nil
}

func TestExecuteTool_WritesToolSpan(t *testing.T) {
    g := NewWithT(t)

    repo := &fakeToolSpanRepo{}
    st := newFakeStore() // 复用 ai_audit_test.go 中的 fakeStore
    svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil, nil)
    service.SetToolSpanRepo(svc, repo)

    _, _ = svc.ExecuteTool(context.Background(), "default", "list_peers", json.RawMessage(`{}`))

    g.Expect(repo.spans).To(HaveLen(1))
    g.Expect(repo.spans[0].Tool).To(Equal("list_peers"))
    g.Expect(repo.spans[0].TraceID).NotTo(BeEmpty())
    g.Expect(repo.spans[0].Status).To(Equal("ok"))
}

func TestExecuteTool_BlockedWritesBlockedSpan(t *testing.T) {
    g := NewWithT(t)

    repo := &fakeToolSpanRepo{}
    st := newFakeStore()
    identity := &fakeAgentIdentityReader{allowedTools: []string{"create_peer"}} // list_peers not allowed
    isolation := service.NewAgentIsolationService(identity, service.AgentIsolationConfig{Mode: "enforce"})
    svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil, isolation)
    service.SetToolSpanRepo(svc, repo)

    ctx := contextWithAgentClaims("agent-x", "default", []string{"create_peer"})
    _, err := svc.ExecuteTool(ctx, "default", "list_peers", json.RawMessage(`{}`))

    g.Expect(err).NotTo(BeNil())
    g.Expect(repo.spans).To(HaveLen(1))
    g.Expect(repo.spans[0].Status).To(Equal("blocked"))
}
```

- [ ] **Step 2: 运行，确认失败**

```bash
go test ./internal/server/service/... -run TestExecuteTool_Writes -v 2>&1
```

期望：FAIL — `service.SetToolSpanRepo` 未定义

- [ ] **Step 3: 在 aiService 添加 toolSpanRepo 字段和 setter**

在 `internal/server/service/ai.go` 中，`aiService` struct 添加字段（在 `tokenCtrl` 之后）：

```go
toolSpanRepo store.ToolSpanRepository // optional, nil = no span recording
```

在文件中添加 setter（紧跟 `SetTokenCtrl` 之后）：

```go
// SetToolSpanRepo attaches a ToolSpanRepository to the AIService for traceID recording.
func SetToolSpanRepo(svc AIService, repo store.ToolSpanRepository) {
    if as, ok := svc.(*aiService); ok {
        as.toolSpanRepo = repo
    }
}
```

在 import 中添加：
```go
"github.com/google/uuid"
```

- [ ] **Step 4: 添加 writeToolSpan 方法**

在 `logToolAudit` 方法之后添加：

> **注意：** `ParentAgentID` 字段在 Sprint 3 Task 3.1 才添加到 `AgentClaims`。此处先留空（Sprint 3 完成后会自动生效，无需修改此方法）。

```go
// writeToolSpan persists a tool call span when toolSpanRepo is configured.
func (s *aiService) writeToolSpan(ctx context.Context, namespace, tool, traceID string,
    start time.Time, status, errMsg string) {
    if s.toolSpanRepo == nil {
        return
    }
    claims := agentClaimsFromContext(ctx)
    agentID, parentID := "", ""
    if claims != nil {
        agentID = claims.AgentID
        // ParentAgentID is populated after Sprint 3 Task 3.1 adds the field to AgentClaims.
        // Reading it here is forward-compatible: zero value = "" before Sprint 3.
        // After Sprint 3: parentID = claims.ParentAgentID (update this line then)
    }
    span := &models.ToolSpan{
        TraceID:    traceID,
        AgentID:    agentID,
        ParentID:   parentID,
        Namespace:  namespace,
        Tool:       tool,
        Status:     status,
        ErrorMsg:   errMsg,
        DurationMs: time.Since(start).Milliseconds(),
        StartedAt:  start,
    }
    _ = s.toolSpanRepo.Write(ctx, span)
}
```

- [ ] **Step 5: 重写 ExecuteTool，提取 dispatchTool，注入 traceID**

将 `ExecuteTool` 中的 `switch name { ... }` 块提取为私有方法 `dispatchTool`，然后重写 `ExecuteTool`：

```go
func (s *aiService) ExecuteTool(ctx context.Context, namespace, name string, input json.RawMessage) (string, error) {
    traceID := uuid.New().String()
    start := time.Now()

    if s.agentIsolation != nil {
        if err := s.agentIsolation.CheckToolAccess(ctx, namespace, name); err != nil {
            s.logToolAudit(ctx, namespace, name, AuditActionAgentToolBlocked)
            s.writeToolSpan(ctx, namespace, name, traceID, start, "blocked", err.Error())
            return "", err
        }
    }
    s.logToolAudit(ctx, namespace, name, AuditActionAgentToolCall)

    result, err := s.dispatchTool(ctx, namespace, name, input)
    status, errMsg := "ok", ""
    if err != nil {
        status, errMsg = "error", err.Error()
    }
    s.writeToolSpan(ctx, namespace, name, traceID, start, status, errMsg)
    return result, err
}

// dispatchTool routes a tool call to its implementation.
// 注意：把原来 ExecuteTool 里 switch name {...} 的全部内容搬到这里，返回值不变。
func (s *aiService) dispatchTool(ctx context.Context, namespace, name string, input json.RawMessage) (string, error) {
    switch name {
    // （此处粘贴原来 ExecuteTool 里 switch 的全部 case，一字不改）
    default:
        return "", fmt.Errorf("unknown tool: %s", name)
    }
}
```

> **重要：** `dispatchTool` 的内容完全来自原 `ExecuteTool` 的 switch 块，不要新增逻辑。

- [ ] **Step 6: 运行测试，确认通过**

```bash
go test ./internal/server/service/... -run TestExecuteTool_Writes -v 2>&1
go test ./internal/server/service/... -v 2>&1
```

期望：全部 PASS

- [ ] **Step 7: Commit**

```bash
git add internal/server/service/ai.go \
        internal/server/service/ai_trace_test.go
git commit -s -m "feat(trace): wrap ExecuteTool with traceID and ToolSpan recording"
```

---

### Task 2.3: 查询 API

**Files:**
- Modify: `internal/server/server/agent_isolation_router.go`
- Modify: `internal/server/server/server.go`

- [ ] **Step 1: 在 server.go 注入 toolSpanRepo 到 AIService**

在 `server.go` 中，找到 aiService 被创建的位置（`NewAIService` 或 `NewAIServiceWithWorkflow` 调用之后），添加：

```go
// 在 agentRegService 初始化之后，找到 aiService 赋值处，添加：
if st != nil {
    service.SetToolSpanRepo(s.aiService, st.ToolSpans())
}
```

（`st` 是 `store.Store` 实例，具体变量名以 server.go 实际代码为准）

- [ ] **Step 2: 在 agentIsolationRouter 添加 /audit/traces 路由**

在 `agent_isolation_router.go` 的 `agentIsolationRouter()` 中添加：

```go
g.GET("/audit/traces",     s.handleListTraces())
g.GET("/audit/traces/:id", s.handleGetTrace())
```

- [ ] **Step 3: 实现 handleListTraces**

```go
// handleListTraces returns a paginated list of tool call spans.
//
// GET /api/v1/agent-isolation/audit/traces?agentId=xxx&from=RFC3339&to=RFC3339&limit=50
func (s *Server) handleListTraces() gin.HandlerFunc {
    return func(c *gin.Context) {
        if s.store == nil {
            resp.PaymentRequired(c, "trace store not configured")
            return
        }
        agentID := c.Query("agentId")
        var from, to time.Time
        if v := c.Query("from"); v != "" {
            t, err := time.Parse(time.RFC3339, v)
            if err != nil {
                resp.BadRequest(c, "invalid from: "+err.Error())
                return
            }
            from = t
        }
        if v := c.Query("to"); v != "" {
            t, err := time.Parse(time.RFC3339, v)
            if err != nil {
                resp.BadRequest(c, "invalid to: "+err.Error())
                return
            }
            to = t
        }
        limit := 50
        if v := c.Query("limit"); v != "" {
            if n, err := strconv.Atoi(v); err == nil && n > 0 {
                limit = n
            }
        }
        spans, err := s.store.ToolSpans().List(c.Request.Context(), agentID, from, to, limit)
        if err != nil {
            resp.Error(c, err.Error())
            return
        }
        resp.OK(c, spans)
    }
}

// handleGetTrace returns a single tool call span by traceId.
//
// GET /api/v1/agent-isolation/audit/traces/:id
func (s *Server) handleGetTrace() gin.HandlerFunc {
    return func(c *gin.Context) {
        if s.store == nil {
            resp.PaymentRequired(c, "trace store not configured")
            return
        }
        traceID := c.Param("id")
        span, err := s.store.ToolSpans().Get(c.Request.Context(), traceID)
        if err != nil {
            resp.Error(c, err.Error())
            return
        }
        resp.OK(c, span)
    }
}
```

在 import 中添加 `"strconv"`。

- [ ] **Step 4: Server struct 需要 store 字段（若不存在）**

检查 `internal/server/server/server.go` 中 `Server` struct，如果没有 `store store.Store` 字段，添加：

```go
store store.Store
```

并在 `NewServer`（或 `New`）中赋值。

- [ ] **Step 5: 编译确认通过**

```bash
go build ./internal/server/... 2>&1
```

期望：无错误

- [ ] **Step 6: Commit**

```bash
git add internal/server/server/agent_isolation_router.go \
        internal/server/server/server.go
git commit -s -m "feat(trace): add GET /api/v1/agent-isolation/audit/traces query API"
```

---

## Sprint 3: Sub-agent Delegate API

**验收标准：** 父 Agent 调用 `POST /api/v1/agent-isolation/delegate`，子 Agent 用返回的 enrollmentToken 注册后，`AgentIdentity.spec.parentRef` 已填写，子 JWT 的 `parent_agent_id` claim 正确。

### Task 3.1: CRD 字段 + AgentClaims 字段

**Files:**
- Modify: `api/v1alpha1/agent_identity_types.go`
- Modify: `internal/server/models/agent_claims.go`
- Modify: `internal/server/models/agent_enrollment.go`

- [ ] **Step 1: 在 AgentIdentitySpec 添加 ParentRef 和 SpawnableRoles**

在 `api/v1alpha1/agent_identity_types.go` 的 `AgentIdentitySpec` struct 中，在 `ExpiresAt` 字段之后添加：

```go
// ParentRef is the name of the parent AgentIdentity (sub-agent scenario).
// Empty means this is a top-level agent.
// +optional
ParentRef string `json:"parentRef,omitempty"`

// SpawnableRoles lists role template names this agent is allowed to spawn as sub-agents.
// Sub-agents spawned from a role template receive that role's permissions,
// regardless of the parent's own permission set.
// +optional
SpawnableRoles []string `json:"spawnableRoles,omitempty"`
```

- [ ] **Step 2: 在 AgentClaims 添加 ParentAgentID**

在 `internal/server/models/agent_claims.go` 的 `AgentClaims` struct 中，在 `AllowedTools` 之后添加：

```go
// ParentAgentID is the AgentID of the parent agent (sub-agent scenario).
// Empty for top-level agents.
ParentAgentID string `json:"parent_agent_id,omitempty"`
```

- [ ] **Step 3: 在 AgentEnrollmentToken 添加 ParentAgentID**

在 `internal/server/models/agent_enrollment.go` 的 `AgentEnrollmentToken` struct 中添加（在 `CreatedBy` 之后）：

```go
// ParentAgentID carries the parent agent's ID through the delegation flow.
// Set by DelegateToken, read by RegisterAgent to populate JWT and AgentIdentity.
ParentAgentID string `gorm:"size:128"`
```

- [ ] **Step 4: 编译确认通过**

```bash
go build ./api/v1alpha1/... ./internal/server/models/... 2>&1
```

期望：无错误

- [ ] **Step 5: Commit**

```bash
git add api/v1alpha1/agent_identity_types.go \
        internal/server/models/agent_claims.go \
        internal/server/models/agent_enrollment.go
git commit -s -m "feat(delegate): add parentRef/spawnableRoles to CRD, ParentAgentID to JWT and token model"
```

---

### Task 3.2: DelegateToken 服务方法

**Files:**
- Modify: `internal/server/service/agent_registration.go`
- Test: `internal/server/service/agent_delegate_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/server/service/agent_delegate_test.go
package service_test

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/alatticeio/lattice/internal/server/models"
    "github.com/alatticeio/lattice/internal/server/service"
    . "github.com/onsi/gomega"
)

func TestDelegateToken_DerivesFromParent(t *testing.T) {
    g := NewWithT(t)

    // 准备：父 Agent JWT（AllowedTools: exec, read）
    st := newFakeStoreWithEnrollmentTokens()
    svc := service.NewAgentRegistrationService("test-secret", st, nil)

    parentJWT := mustIssueAgentJWT(svc, "parent-agent", "wf-ns", []string{"exec", "read"})

    resp, err := svc.DelegateToken(context.Background(), service.DelegateRequest{
        ParentJWT:      parentJWT,
        AgentName:      "child-001",
        RequestedTools: []string{"read"}, // 子集 ⊆ 父工具
    })
    g.Expect(err).To(BeNil())
    g.Expect(resp.EnrollmentToken).NotTo(BeEmpty())
    g.Expect(resp.ExpiresAt).To(BeTemporally(">", time.Now()))

    // 验证 enrollment token 已存储且携带 ParentAgentID
    tok, err := st.AgentEnrollmentTokens().GetByToken(context.Background(), resp.EnrollmentToken)
    g.Expect(err).To(BeNil())
    g.Expect(tok.ParentAgentID).To(Equal("parent-agent"))

    // 解析 AllowedTools
    var tools []string
    _ = json.Unmarshal([]byte(tok.AllowedTools), &tools)
    g.Expect(tools).To(ConsistOf("read"))
}

func TestDelegateToken_RejectsToolsOutsideParent(t *testing.T) {
    g := NewWithT(t)

    st := newFakeStoreWithEnrollmentTokens()
    svc := service.NewAgentRegistrationService("test-secret", st, nil)

    parentJWT := mustIssueAgentJWT(svc, "parent-agent", "wf-ns", []string{"read"})

    _, err := svc.DelegateToken(context.Background(), service.DelegateRequest{
        ParentJWT:      parentJWT,
        AgentName:      "child-001",
        RequestedTools: []string{"exec"}, // exec 不在父的工具里
    })
    g.Expect(err).NotTo(BeNil())
    g.Expect(err.Error()).To(ContainSubstring("not permitted"))
}

// mustIssueAgentJWT 是测试辅助函数，通过反射或直接调用 issueAgentJWT。
// 实际实现：在 agent_registration.go 中导出一个测试专用的 IssueAgentJWTForTest 函数，
// 或者直接用 CreateEnrollmentToken + RegisterAgent 流程（但需要 k8s client）。
// 更简单的做法：在 agentRegistrationService 上暴露 issueAgentJWT 为包级函数（仅用于测试）。
func mustIssueAgentJWT(svc service.AgentRegistrationService, agentID, ns string, tools []string) string {
    // 利用 ValidateAgentJWT 的逆向：直接用 jwt.NewWithClaims 签一个 token。
    // 避免修改 svc 接口，直接构造：
    claims := &models.AgentClaims{
        AgentID:      agentID,
        Namespace:    ns,
        AllowedTools: tools,
    }
    // 使用测试密钥 "test-secret" 签名
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, _ := token.SignedString([]byte("test-secret"))
    return signed
}
```

- [ ] **Step 2: 运行，确认失败**

```bash
go test ./internal/server/service/... -run TestDelegateToken -v 2>&1
```

期望：FAIL — `service.DelegateToken` 未定义 / `service.DelegateRequest` 未定义

- [ ] **Step 3: 在 AgentRegistrationService 接口添加 DelegateToken**

在 `agent_registration.go` 的 `AgentRegistrationService` interface 中添加：

```go
// DelegateToken creates a short-lived enrollment token for a sub-agent.
// The parent's JWT is validated; the sub-agent's tools must be a subset of
// the parent's tools (or come from an admin-authorized SpawnableRole).
DelegateToken(ctx context.Context, req DelegateRequest) (*DelegateResponse, error)
```

添加 DelegateRequest / DelegateResponse 类型（放在 `AgentRegisterResponse` 之后）：

```go
// DelegateRequest is the input for creating a sub-agent enrollment token.
type DelegateRequest struct {
    // ParentJWT is the parent agent's signed JWT.
    ParentJWT string
    // AgentName is the desired name for the sub-agent.
    AgentName string
    // RequestedTools is the tool whitelist for the sub-agent.
    // Must be a subset of the parent's AllowedTools (when RoleName is empty).
    RequestedTools []string
    // RoleName, if non-empty, uses the parent's SpawnableRoles path.
    // The sub-agent receives the role template's permissions (may exceed parent's).
    // The server validates that parent.SpawnableRoles contains RoleName.
    RoleName string
}

// DelegateResponse is the result of a successful DelegateToken call.
type DelegateResponse struct {
    // EnrollmentToken is a short-lived one-time token for the sub-agent to register.
    EnrollmentToken string    `json:"enrollmentToken"`
    ExpiresAt       time.Time `json:"expiresAt"`
}
```

- [ ] **Step 4: 实现 DelegateToken 方法**

在 `agentRegistrationService` 上添加实现：

```go
func (s *agentRegistrationService) DelegateToken(ctx context.Context, req DelegateRequest) (*DelegateResponse, error) {
    // 1. 验证父 Agent JWT
    parentClaims, err := s.ValidateAgentJWT(req.ParentJWT)
    if err != nil {
        return nil, fmt.Errorf("invalid parent JWT: %w", err)
    }

    var allowedTools []string

    if req.RoleName != "" {
        // SpawnableRoles 路径：子权限可大于父，由管理员预授权
        if s.k8s == nil {
            return nil, fmt.Errorf("k8s client required for SpawnableRoles")
        }
        // 查父 AgentIdentity
        parentIdentity := &v1alpha1.AgentIdentity{}
        if err := s.k8s.Get(ctx, k8sclient.ObjectKey{
            Namespace: parentClaims.Namespace,
            Name:      parentClaims.AgentID,
        }, parentIdentity); err != nil {
            return nil, fmt.Errorf("lookup parent AgentIdentity: %w", err)
        }
        // 校验 SpawnableRoles 包含 req.RoleName
        found := false
        for _, r := range parentIdentity.Spec.SpawnableRoles {
            if r == req.RoleName {
                found = true
                break
            }
        }
        if !found {
            return nil, fmt.Errorf("role %q not in parent's spawnableRoles", req.RoleName)
        }
        // 此处从角色模板取 allowedTools — 角色模板使用 req.RoleName 命名的 AgentIdentity（约定）
        roleIdentity := &v1alpha1.AgentIdentity{}
        if err := s.k8s.Get(ctx, k8sclient.ObjectKey{
            Namespace: parentClaims.Namespace,
            Name:      req.RoleName,
        }, roleIdentity); err != nil {
            return nil, fmt.Errorf("lookup role template %q: %w", req.RoleName, err)
        }
        allowedTools = roleIdentity.Spec.AllowedTools
    } else {
        // 派生路径：子权限 ⊆ 父权限
        parentToolSet := make(map[string]bool, len(parentClaims.AllowedTools))
        for _, t := range parentClaims.AllowedTools {
            parentToolSet[t] = true
        }
        for _, t := range req.RequestedTools {
            if !parentToolSet[t] {
                return nil, fmt.Errorf("tool %q not permitted by parent", t)
            }
        }
        allowedTools = req.RequestedTools
    }

    // 2. 生成一次性 enrollment token（TTL=15min），携带 parentAgentID
    raw := make([]byte, 32)
    if _, err := rand.Read(raw); err != nil {
        return nil, fmt.Errorf("generate token: %w", err)
    }
    token := hex.EncodeToString(raw)

    toolsJSON, _ := json.Marshal(allowedTools)
    expiresAt := time.Now().Add(15 * time.Minute)

    tok := &models.AgentEnrollmentToken{
        Token:            token,
        AllowedNamespace: parentClaims.Namespace,
        AllowedTools:     string(toolsJSON),
        ExpiresAt:        expiresAt,
        CreatedBy:        parentClaims.AgentID,
        ParentAgentID:    parentClaims.AgentID,
    }
    if err := s.store.AgentEnrollmentTokens().Create(ctx, tok); err != nil {
        return nil, fmt.Errorf("store delegate token: %w", err)
    }

    s.logger.Info("delegate token created", "parent", parentClaims.AgentID, "child", req.AgentName)
    return &DelegateResponse{EnrollmentToken: token, ExpiresAt: expiresAt}, nil
}
```

- [ ] **Step 5: 更新 writeToolSpan 以填入 ParentAgentID**

Sprint 3 完成后，在 `internal/server/service/ai.go` 的 `writeToolSpan` 中，将注释行替换为：

```go
if claims != nil {
    agentID = claims.AgentID
    parentID = claims.ParentAgentID  // 现在字段存在了
}
```

- [ ] **Step 6: 更新 RegisterAgent 以读取 ParentAgentID**

在 `RegisterAgent` 中，step 4（创建 AgentIdentity）之前读取 `tok.ParentAgentID`，然后在 AgentIdentity 中设置：

```go
// 在 identity.Spec 中添加（在 Sandbox 赋值之后）：
if tok.ParentAgentID != "" {
    identity.Spec.ParentRef = tok.ParentAgentID
}
```

在 step 6（issueAgentJWT）之前，修改为携带 ParentAgentID 的版本：

```go
// 将原来的 s.issueAgentJWT(req.AgentName, tok.AllowedNamespace, allowedTools)
// 改为：
agentJWT, err := s.issueAgentJWT(req.AgentName, tok.AllowedNamespace, allowedTools, tok.ParentAgentID)
```

更新 `issueAgentJWT` 签名，添加 `parentAgentID string` 参数：

```go
func (s *agentRegistrationService) issueAgentJWT(agentName, namespace string, allowedTools []string, parentAgentID string) (string, error) {
    now := time.Now()
    claims := &models.AgentClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   agentName,
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(365 * 24 * time.Hour)),
            Issuer:    "lattice-agent-registration",
        },
        AgentID:       agentName,
        Namespace:     namespace,
        AllowedTools:  allowedTools,
        ParentAgentID: parentAgentID,
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(s.jwtSecret))
}
```

- [ ] **Step 6: 运行测试，确认通过**

```bash
go test ./internal/server/service/... -run TestDelegateToken -v 2>&1
```

期望：PASS

- [ ] **Step 7: 编译确认通过**

```bash
go build ./... 2>&1
```

期望：无错误

- [ ] **Step 9: Commit**

```bash
git add internal/server/service/agent_registration.go \
        internal/server/service/agent_delegate_test.go
git commit -s -m "feat(delegate): implement DelegateToken with parent JWT validation and SpawnableRoles"
```

---

### Task 3.3: Delegate HTTP Endpoint

**Files:**
- Modify: `internal/server/server/agent_isolation_router.go`

- [ ] **Step 1: 在 agentIsolationRouter 添加路由**

在 `agentIsolationRouter()` 中，在现有路由之后添加：

```go
g.POST("/delegate", s.handleDelegate())
```

- [ ] **Step 2: 实现 handleDelegate**

```go
// handleDelegate creates a sub-agent enrollment token from a parent Agent JWT.
//
// POST /api/v1/agent-isolation/delegate
// Authorization: Bearer <parent-agent-jwt>
// Body: { "agentName": "sub-001", "requestedTools": ["read"], "roleName": "" }
func (s *Server) handleDelegate() gin.HandlerFunc {
    return func(c *gin.Context) {
        if s.agentRegService == nil {
            resp.PaymentRequired(c, "agent isolation is not enabled")
            return
        }
        // 从 Authorization: Bearer 头取父 JWT
        authHeader := c.GetHeader("Authorization")
        if !strings.HasPrefix(authHeader, "Bearer ") {
            resp.Unauthorized(c, "Bearer token required")
            return
        }
        parentJWT := strings.TrimPrefix(authHeader, "Bearer ")

        var req struct {
            AgentName      string   `json:"agentName"      binding:"required"`
            RequestedTools []string `json:"requestedTools"`
            RoleName       string   `json:"roleName"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            resp.BadRequest(c, "invalid request: "+err.Error())
            return
        }
        result, err := s.agentRegService.DelegateToken(c.Request.Context(), service.DelegateRequest{
            ParentJWT:      parentJWT,
            AgentName:      req.AgentName,
            RequestedTools: req.RequestedTools,
            RoleName:       req.RoleName,
        })
        if err != nil {
            resp.Error(c, err.Error())
            return
        }
        resp.OK(c, result)
    }
}
```

在 import 中添加 `"strings"`（如不存在）。

- [ ] **Step 3: 编译确认通过**

```bash
go build ./internal/server/... 2>&1
```

期望：无错误

- [ ] **Step 4: Commit**

```bash
git add internal/server/server/agent_isolation_router.go
git commit -s -m "feat(delegate): add POST /api/v1/agent-isolation/delegate HTTP endpoint"
```

---

## Sprint 4: gVisor AuditWriter → 控制面上报（PRO）

**验收标准（PRO build）：** 在 PRO 版本中，gVisor sandbox 的网络流量事件通过 NATS 上报到 latticed，并存储在 `flow_events` 表中。

### Task 4.1: FlowEvent 模型 + 存储层

**Files:**
- Create: `internal/server/models/flow_event.go`
- Create: `internal/db/gormstore/flow_event.go`
- Modify: `internal/db/gormstore/migrate.go`
- Modify: `internal/agent/store/store.go`
- Modify: `internal/db/gormstore/store.go`
- Test: `internal/db/gormstore/flow_event_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/db/gormstore/flow_event_test.go
package gormstore_test

import (
    "context"
    "testing"
    "time"

    "github.com/alatticeio/lattice/internal/db/gormstore"
    "github.com/alatticeio/lattice/internal/server/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupFlowEventDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    _ = db.AutoMigrate(&models.FlowEvent{})
    return db
}

func TestFlowEvent_WriteAndListByTrace(t *testing.T) {
    db := setupFlowEventDB(t)
    repo := gormstore.NewFlowEventRepo(db)
    ctx := context.Background()

    _ = repo.Write(ctx, &models.FlowEvent{
        TraceID: "trace-001", AgentID: "agent-a",
        Direction: "egress", DstIP: "10.0.0.1", DstPort: 443, Bytes: 1024,
        Ts: time.Now().UTC(),
    })

    events, err := repo.ListByTrace(ctx, "trace-001")
    if err != nil {
        t.Fatalf("ListByTrace: %v", err)
    }
    if len(events) != 1 {
        t.Errorf("expected 1 event, got %d", len(events))
    }
    if events[0].DstPort != 443 {
        t.Errorf("port: got %d, want 443", events[0].DstPort)
    }
}
```

- [ ] **Step 2: 运行，确认失败**

```bash
go test ./internal/db/gormstore/... -run TestFlowEvent -v 2>&1
```

期望：FAIL — `gormstore.NewFlowEventRepo` 未定义

- [ ] **Step 3: 创建 FlowEvent 模型**

```go
// internal/server/models/flow_event.go
package models

import "time"

// FlowEvent records a single network flow observed by the gVisor AuditWriter.
// Linked to ToolSpan via TraceID for tool call → network traffic correlation.
type FlowEvent struct {
    Model
    TraceID   string    `gorm:"index;size:36"  json:"traceId"`
    AgentID   string    `gorm:"index;size:128" json:"agentId"`
    Direction string    `gorm:"size:16"        json:"direction"` // egress | ingress
    DstIP     string    `gorm:"size:64"        json:"dstIp"`
    DstPort   int       `json:"dstPort"`
    Bytes     int64     `json:"bytes"`
    Ts        time.Time `gorm:"index"          json:"ts"`
}

func (FlowEvent) TableName() string { return "la_flow_events" }
```

- [ ] **Step 4: 创建 FlowEventRepo 实现**

```go
// internal/db/gormstore/flow_event.go
package gormstore

import (
    "context"

    "github.com/alatticeio/lattice/internal/server/models"
    "gorm.io/gorm"
)

type flowEventRepo struct{ db *gorm.DB }

func NewFlowEventRepo(db *gorm.DB) *flowEventRepo {
    return &flowEventRepo{db: db}
}

func (r *flowEventRepo) Write(ctx context.Context, e *models.FlowEvent) error {
    return r.db.WithContext(ctx).Create(e).Error
}

func (r *flowEventRepo) ListByTrace(ctx context.Context, traceID string) ([]*models.FlowEvent, error) {
    var events []*models.FlowEvent
    return events, r.db.WithContext(ctx).
        Where("trace_id = ?", traceID).
        Order("ts asc").
        Find(&events).Error
}
```

- [ ] **Step 5: 在 store.go 接口添加 FlowEvents()**

在 `internal/agent/store/store.go` 中，`Store` interface 添加：

```go
FlowEvents() FlowEventRepository
```

添加 `FlowEventRepository` 接口（在 `ToolSpanRepository` 之后）：

```go
// FlowEventRepository records and queries gVisor network flow events.
type FlowEventRepository interface {
    Write(ctx context.Context, e *models.FlowEvent) error
    ListByTrace(ctx context.Context, traceID string) ([]*models.FlowEvent, error)
}
```

- [ ] **Step 6: 在 gormstore/migrate.go 和 store.go 注册**

`migrate.go` 添加 `&models.FlowEvent{}`；`store.go` 添加 `flowEvents` 字段、`NewFlowEventRepo(db)` 初始化、`FlowEvents()` 方法（模式与 `ToolSpans` 完全相同）。

- [ ] **Step 7: 运行测试，确认通过**

```bash
go test ./internal/db/gormstore/... -run TestFlowEvent -v 2>&1
go build ./... 2>&1
```

期望：PASS，无编译错误

- [ ] **Step 8: Commit**

```bash
git add internal/server/models/flow_event.go \
        internal/db/gormstore/flow_event.go \
        internal/db/gormstore/flow_event_test.go \
        internal/db/gormstore/migrate.go \
        internal/db/gormstore/store.go \
        internal/agent/store/store.go
git commit -s -m "feat(audit): add FlowEvent model and GORM repository"
```

---

### Task 4.2: PRO natsAuditWriter（替换 fileAuditWriter）

**Files:**
- Modify: `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` (build tag: pro)

> 此 Task 仅影响 PRO edition，community edition 不受影响。

- [ ] **Step 1: 确认 shim.AuditEvent 的字段名**

先读取 shim 库的 AuditEvent 类型定义，确认字段名：

```bash
grep -r "AuditEvent" $(go env GOPATH)/pkg/mod/github.com/alatticeio/lattice-shim*/shim/ 2>/dev/null | head -20
# 或直接查看 vendor 目录（若有）
```

期望输出类似：
```
type AuditEvent struct {
    Direction string
    DstIP     string
    DstPort   uint16
    Bytes     int64
    // ... 其他字段
}
```

**根据实际字段名调整下方代码中 `event.XXX` 的访问路径。**

- [ ] **Step 2: 在 start_sandbox_pro.go 中添加 natsAuditWriter**

在 `fileAuditWriter` 类型之后添加（字段名以 Step 1 确认的为准）：

```go
// natsAuditWriter publishes audit events to the Lattice control plane via NATS.
// Each event carries the active traceID for correlation with ToolSpan records.
type natsAuditWriter struct {
    nc      *nats.Conn
    agentID string
}

func (w *natsAuditWriter) WriteAudit(agentID string, event shim.AuditEvent) error {
    type flowMsg struct {
        AgentID   string `json:"agentId"`
        TraceID   string `json:"traceId"`
        Direction string `json:"direction"`
        DstIP     string `json:"dstIp"`
        DstPort   int    `json:"dstPort"`
        Bytes     int64  `json:"bytes"`
        Ts        string `json:"ts"`
    }
    payload, err := json.Marshal(flowMsg{
        AgentID:   agentID,
        TraceID:   activeTraceID(),
        Direction: event.Direction,            // 根据 Step 1 调整
        DstIP:     event.DstIP,               // 根据 Step 1 调整
        DstPort:   int(event.DstPort),        // 根据 Step 1 调整
        Bytes:     event.Bytes,               // 根据 Step 1 调整
        Ts:        time.Now().UTC().Format(time.RFC3339Nano),
    })
    if err != nil {
        return err
    }
    return w.nc.Publish("lattice.audit.flow", payload)
}

// activeTraceID returns the current tool call traceID.
// The traceID is injected by the agent process via env var LATTICE_CURRENT_TRACE_ID
// before each tool dispatch. Empty string means the flow is outside a tool call.
func activeTraceID() string {
    return os.Getenv("LATTICE_CURRENT_TRACE_ID")
}
```

在 import 中添加（PRO build 已有 `os`，需添加）：
```go
nats "github.com/nats-io/nats.go"
"time"
```

- [ ] **Step 2: 修改 createSandbox 以支持 NATS audit writer**

在 `createSandbox` 函数签名中添加可选 NATS 连接参数，或通过环境变量检测：

```go
// 在 createSandbox 中，替换 fileAuditWriter 的构建逻辑：

// 默认：本地文件审计（fallback）
var auditWriter shim.AuditWriter
auditPath := "/tmp/lattice-audit.jsonl"
f, err := os.OpenFile(auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
if err != nil {
    log.Printf("[sandbox] cannot open audit file %s: %v", auditPath, err)
} else {
    auditFile = f
    auditWriter = gvisor.NewAuditAdapter(sandboxName, &fileAuditWriter{f: f})
}

// PRO 增强：若 NATS URL 可用，升级为 natsAuditWriter
if natsURL := os.Getenv("LATTICE_NATS_URL"); natsURL != "" {
    nc, err := nats.Connect(natsURL)
    if err != nil {
        log.Printf("[sandbox] NATS connect failed: %v, falling back to file audit", err)
    } else {
        natsWriter := &natsAuditWriter{nc: nc, agentID: sandboxName}
        if auditFile != nil {
            // 同时写文件和 NATS（双写保障）
            auditWriter = gvisor.NewAuditAdapter(sandboxName, &fileAuditWriter{f: auditFile}, natsWriter)
        } else {
            auditWriter = gvisor.NewAuditAdapter(sandboxName, natsWriter)
        }
        log.Printf("[sandbox] NATS audit writer connected to %s", natsURL)
    }
}
```

- [ ] **Step 3: 确认 PRO 编译通过**

```bash
go build -tags pro ./cmd/lattice-agent-sandbox/... 2>&1
```

期望：无错误

- [ ] **Step 4: Commit**

```bash
git add cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go
git commit -s -m "feat(audit): add natsAuditWriter for PRO gVisor flow event upload"
```

---

### Task 4.3: 控制面 NATS 订阅器（PRO）

**Files:**
- Create: `internal/server/controller/audit_consumer.go` (build tag: pro)

- [ ] **Step 1: 创建 NATS audit consumer**

```go
//go:build pro

// internal/server/controller/audit_consumer.go
package controller

import (
    "context"
    "encoding/json"
    "log"
    "time"

    "github.com/alatticeio/lattice/internal/agent/store"
    "github.com/alatticeio/lattice/internal/server/models"
    "github.com/nats-io/nats.go"
)

// FlowAuditMsg is the wire format published by natsAuditWriter.
type FlowAuditMsg struct {
    AgentID   string `json:"agentId"`
    TraceID   string `json:"traceId"`
    Direction string `json:"direction"`
    DstIP     string `json:"dstIp"`
    DstPort   int    `json:"dstPort"`
    Bytes     int64  `json:"bytes"`
    Ts        string `json:"ts"`
}

// AuditConsumer subscribes to NATS audit topics and persists flow events.
type AuditConsumer struct {
    nc         *nats.Conn
    flowEvents store.FlowEventRepository
    sub        *nats.Subscription
}

// NewAuditConsumer creates and starts the NATS flow audit consumer.
func NewAuditConsumer(nc *nats.Conn, flowEvents store.FlowEventRepository) (*AuditConsumer, error) {
    c := &AuditConsumer{nc: nc, flowEvents: flowEvents}
    sub, err := nc.Subscribe("lattice.audit.flow", c.handleFlowEvent)
    if err != nil {
        return nil, err
    }
    c.sub = sub
    log.Printf("[audit-consumer] subscribed to lattice.audit.flow")
    return c, nil
}

func (c *AuditConsumer) handleFlowEvent(msg *nats.Msg) {
    var m FlowAuditMsg
    if err := json.Unmarshal(msg.Data, &m); err != nil {
        log.Printf("[audit-consumer] unmarshal error: %v", err)
        return
    }
    ts, _ := time.Parse(time.RFC3339Nano, m.Ts)
    if ts.IsZero() {
        ts = time.Now().UTC()
    }
    e := &models.FlowEvent{
        TraceID:   m.TraceID,
        AgentID:   m.AgentID,
        Direction: m.Direction,
        DstIP:     m.DstIP,
        DstPort:   m.DstPort,
        Bytes:     m.Bytes,
        Ts:        ts,
    }
    if err := c.flowEvents.Write(context.Background(), e); err != nil {
        log.Printf("[audit-consumer] write flow event: %v", err)
    }
}

// Close unsubscribes from NATS.
func (c *AuditConsumer) Close() {
    if c.sub != nil {
        _ = c.sub.Unsubscribe()
    }
}
```

- [ ] **Step 2: 在 Server 初始化时启动 AuditConsumer（PRO only）**

在 `internal/server/server/server.go` 中（PRO build），找到 NATS 连接初始化之后，添加：

```go
// 在 server.go 中，NATS client 初始化后：
if nc != nil && st != nil {
    consumer, err := controller.NewAuditConsumer(nc, st.FlowEvents())
    if err != nil {
        s.logger.Warn("audit consumer failed to start", "err", err)
    } else {
        s.auditConsumer = consumer
    }
}
```

在 `Server` struct 中添加字段：
```go
auditConsumer *controller.AuditConsumer // PRO only, nil in community
```

在 `Server.Close()` 或 shutdown 逻辑中：
```go
if s.auditConsumer != nil {
    s.auditConsumer.Close()
}
```

- [ ] **Step 3: 确认 PRO 编译通过**

```bash
go build -tags pro ./internal/server/... 2>&1
go build -tags pro ./cmd/latticed/... 2>&1
```

期望：无错误

- [ ] **Step 4: Commit**

```bash
git add internal/server/controller/audit_consumer.go \
        internal/server/server/server.go
git commit -s -m "feat(audit): add NATS flow audit consumer for PRO control plane"
```

---

## 最终验收

- [ ] Community build 编译、E2E 沙箱测试通过：
  ```bash
  go build ./... 2>&1
  make test-e2e E2E_KUBECONFIG=/tmp/lattice-e2e.kubeconfig 2>&1
  ```

- [ ] PRO build 编译：
  ```bash
  go build -tags pro ./... 2>&1
  ```

- [ ] 单元测试全部通过：
  ```bash
  go test ./internal/server/service/... ./internal/db/gormstore/... -v 2>&1
  ```

- [ ] 工具调用可查询（手动验证）：
  ```bash
  curl -s "http://localhost:8080/api/v1/agent-isolation/audit/traces?agentId=test-agent" \
    -H "Authorization: Bearer $TOKEN" | jq .
  ```
