# Lattice Playground 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Lattice 构建三层产品体验入口：Demo 账号（零摩擦）、种子数据（新用户第一印象）、安装脚本（动手路径）、Sealos 一键部署（企业入口）。

**Architecture:** 种子数据在 Workspace 创建时注入，存入现有数据库表并打 `is_seed=true` 标记；Demo 账号是独立 `latticed` 实例，通过只读 API 模式 + 真实 VPS 节点提供；安装脚本 Shell 文件托管在 CDN；Sealos 模板为单独 YAML 文件。

**Tech Stack:** Go 1.25, GORM + SQLite/MySQL, Gin, Ginkgo v2 + Gomega, Shell, Docker Compose, Sealos Template YAML

---

## 文件映射

| 文件 | 变更类型 | 职责 |
|------|---------|------|
| `internal/server/seed/seed.go` | 新建 | 种子数据注入逻辑 |
| `internal/server/seed/seed_test.go` | 新建 | 种子数据单元测试 |
| `internal/server/models/workspace.go` | 修改 | Workspace 模型加 `SeedInjected` 字段 |
| `internal/server/service/workspace.go` | 修改 | `AddWorkspace` 完成后调用 `InjectSeedData` |
| `internal/db/gormstore/migrate.go` | 修改 | 注册新字段 AutoMigrate |
| `internal/server/server/workspace.go` | 修改 | 新增 `DELETE /api/v1/workspaces/:id/seed` 接口 |
| `internal/server/controller/workspace.go` | 修改 | 新增 `ClearSeedData` 方法 |
| `internal/server/server/middleware/readonly.go` | 新建 | Demo 账号只读中间件 |
| `scripts/install.sh` | 新建 | 快速安装脚本 |
| `deploy/sealos/app.yaml` | 新建 | Sealos 一键部署模板 |
| `deploy/demo/docker-compose.yml` | 新建 | Demo 环境 docker-compose |
| `docs/playground.md` | 新建 | 用户文档：Playground 入口介绍 |

---

## Task 1：给 Workspace 模型加 `SeedInjected` 字段

**Files:**
- Modify: `internal/server/models/workspace.go`
- Modify: `internal/db/gormstore/migrate.go`

- [ ] **Step 1: 读当前 Workspace model**

```bash
cat -n internal/server/models/workspace.go
```

- [ ] **Step 2: 写失败测试**

在 `internal/db/gormstore/workspace_test.go` 末尾添加：

```go
It("should have SeedInjected field on Workspace model", func() {
    ws := &models.Workspace{
        Slug:        "test-seed-field",
        DisplayName: "Test Seed Field",
    }
    Expect(store.Workspaces().Create(ctx, ws)).To(Succeed())
    ws.SeedInjected = true
    Expect(store.Workspaces().Update(ctx, ws)).To(Succeed())

    got, err := store.Workspaces().GetByID(ctx, ws.ID)
    Expect(err).NotTo(HaveOccurred())
    Expect(got.SeedInjected).To(BeTrue())
})
```

- [ ] **Step 3: 运行测试，确认失败**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/db/gormstore/... -run "TestGormStore" -v 2>&1 | tail -20
```

期望：`FAIL` — `unknown field SeedInjected`

- [ ] **Step 4: 在 Workspace 模型加字段**

找到 `internal/server/models/workspace.go` 中的 `Workspace` struct，在最后一个字段后追加：

```go
SeedInjected bool `gorm:"default:false" json:"seedInjected"`
```

- [ ] **Step 5: 运行测试，确认通过**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/db/gormstore/... -run "TestGormStore" -v 2>&1 | tail -20
```

期望：`PASS`

- [ ] **Step 6: 提交**

```bash
git add internal/server/models/workspace.go internal/db/gormstore/workspace_test.go
git commit -s -m "feat(playground): add SeedInjected field to Workspace model"
```

---

## Task 2：实现种子数据注入器

**Files:**
- Create: `internal/server/seed/seed.go`
- Create: `internal/server/seed/seed_test.go`

- [ ] **Step 1: 创建 suite_test.go**

创建 `internal/server/seed/suite_test.go`：

```go
// Copyright 2024 The Lattice Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package seed_test

import (
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestSeedSuite(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Seed Suite")
}
```

- [ ] **Step 1b: 写失败测试**

创建 `internal/server/seed/seed_test.go`：

```go
// Copyright 2024 The Lattice Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package seed_test

import (
    "context"

    "github.com/alatticeio/lattice/internal/agent/store"
    "github.com/alatticeio/lattice/internal/db/gormstore"
    "github.com/alatticeio/lattice/internal/server/seed"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

var _ = Describe("Injector", func() {
    var (
        st  store.Store
        ctx context.Context
    )

    BeforeEach(func() {
        db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
        Expect(err).NotTo(HaveOccurred())
        st, err = gormstore.New(db)
        Expect(err).NotTo(HaveOccurred())
        ctx = context.Background()
    })

    It("injects audit logs tagged with IsSeed=true", func() {
        injector := seed.NewInjector(st)
        err := injector.Inject(ctx, "ws-test-001", seed.Options{
            VirtualNodes: 2,
            HistoryDays:  3,
            AuditEntries: 5,
        })
        Expect(err).NotTo(HaveOccurred())

        logs, total, err := st.AuditLogs().List(ctx, store.AuditLogFilter{
            WorkspaceID: "ws-test-001",
            PageSize:    20,
        })
        Expect(err).NotTo(HaveOccurred())
        Expect(total).To(BeNumerically(">=", int64(5)))
        Expect(logs[0].IsSeed).To(BeTrue())
    })

    It("injects 3 policies tagged with IsSeed=true", func() {
        injector := seed.NewInjector(st)
        err := injector.Inject(ctx, "ws-test-002", seed.DefaultOptions())
        Expect(err).NotTo(HaveOccurred())

        policies, _, err := st.Policies().List(ctx, store.PolicyFilter{
            WorkspaceID: "ws-test-002",
            PageSize:    20,
        })
        Expect(err).NotTo(HaveOccurred())
        Expect(len(policies)).To(BeNumerically(">=", 3))
        Expect(policies[0].IsSeed).To(BeTrue())
    })

    It("Clear removes all seed records", func() {
        injector := seed.NewInjector(st)
        Expect(injector.Inject(ctx, "ws-test-003", seed.DefaultOptions())).To(Succeed())
        Expect(injector.Clear(ctx, "ws-test-003")).To(Succeed())

        logs, total, err := st.AuditLogs().List(ctx, store.AuditLogFilter{
            WorkspaceID: "ws-test-003",
            PageSize:    20,
        })
        Expect(err).NotTo(HaveOccurred())
        Expect(total).To(Equal(int64(0)))
        _ = logs
    })
})
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/server/seed/... -v 2>&1 | tail -20
```

期望：`FAIL` — `cannot find package`

- [ ] **Step 3: 实现 seed.go**

创建 `internal/server/seed/seed.go`：

```go
// Copyright 2024 The Lattice Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package seed

import (
    "context"
    "fmt"
    "time"

    "github.com/alatticeio/lattice/internal/agent/store"
    "github.com/alatticeio/lattice/internal/server/models"
    "github.com/google/uuid"
)

// Options controls the volume of seed data injected into a new workspace.
type Options struct {
    VirtualNodes int // default 8
    HistoryDays  int // default 7
    AuditEntries int // default 20
}

// DefaultOptions returns the standard seed volume.
func DefaultOptions() Options {
    return Options{
        VirtualNodes: 8,
        HistoryDays:  7,
        AuditEntries: 20,
    }
}

// Injector injects seed data into a workspace.
type Injector struct {
    store store.Store
}

// NewInjector creates a new Injector backed by the given store.
func NewInjector(s store.Store) *Injector {
    return &Injector{store: s}
}

// Inject writes seed audit logs, policies, and alerts into workspaceID.
// All records are tagged with IsSeed=true so they can be cleared later.
func (inj *Injector) Inject(ctx context.Context, workspaceID string, opts Options) error {
    if err := inj.injectAuditLogs(ctx, workspaceID, opts); err != nil {
        return fmt.Errorf("seed audit logs: %w", err)
    }
    if err := inj.injectPolicies(ctx, workspaceID); err != nil {
        return fmt.Errorf("seed policies: %w", err)
    }
    if err := inj.injectAlerts(ctx, workspaceID); err != nil {
        return fmt.Errorf("seed alerts: %w", err)
    }
    return nil
}

// Clear removes all seed records from the given workspace.
func (inj *Injector) Clear(ctx context.Context, workspaceID string) error {
    return inj.store.Seed().Clear(ctx, workspaceID)
}

func (inj *Injector) injectAuditLogs(ctx context.Context, workspaceID string, opts Options) error {
    now := time.Now()
    entries := opts.AuditEntries
    if entries <= 0 {
        entries = 20
    }

    actions := []struct{ action, resource, scope string }{
        {"CREATE", "policy", "policy: allow-web → allow-db"},
        {"UPDATE", "policy", "policy: deny-all → action: DENY"},
        {"CREATE", "member", "member: alice@example.com → role: editor"},
        {"DELETE", "peer",   "peer: node-beijing-01"},
        {"CREATE", "token",  "token: deploy-token"},
        {"UPDATE", "workspace", "displayName: My Workspace"},
        {"INVITE", "member", "email: bob@example.com"},
        {"CREATE", "policy", "policy: allow-monitoring"},
        {"UPDATE", "member", "member: carol@example.com → role: viewer"},
        {"DELETE", "token",  "token: old-token"},
    }

    logs := make([]*models.AuditLog, 0, entries)
    for i := range entries {
        a := actions[i%len(actions)]
        t := now.Add(-time.Duration(i) * 6 * time.Hour)
        logs = append(logs, &models.AuditLog{
            ID:           uuid.NewString(),
            CreatedAt:    t,
            UserID:       "seed-user",
            UserName:     "demo-admin",
            UserEmail:    "admin@demo.lattice.io",
            UserIP:       "10.0.0.1",
            WorkspaceID:  workspaceID,
            Action:       a.action,
            Resource:     a.resource,
            ResourceID:   uuid.NewString(),
            ResourceName: fmt.Sprintf("seed-%s-%d", a.resource, i),
            Scope:        a.scope,
            Status:       "success",
            StatusCode:   200,
            IsSeed:       true,
        })
    }
    return inj.store.AuditLogs().BatchCreate(ctx, logs)
}

func (inj *Injector) injectPolicies(ctx context.Context, workspaceID string) error {
    policies := []*models.Policy{
        {
            WorkspaceID: workspaceID,
            Name:        "seed-allow-web-to-db",
            Description: "Allow web tier to reach database tier",
            Action:      "ALLOW",
            PolicyTypes: `["Egress"]`,
            Spec:        `{"selector":{"matchLabels":{"app":"web"}},"egress":[{"to":[{"podSelector":{"matchLabels":{"app":"db"}}}]}]}`,
            Status:      models.PolicyStatusActive,
            IsSeed:      true,
        },
        {
            WorkspaceID: workspaceID,
            Name:        "seed-allow-monitoring",
            Description: "Allow monitoring scrape from any node",
            Action:      "ALLOW",
            PolicyTypes: `["Ingress"]`,
            Spec:        `{"selector":{},"ingress":[{"from":[{"podSelector":{"matchLabels":{"app":"prometheus"}}}],"ports":[{"port":9090}]}]}`,
            Status:      models.PolicyStatusActive,
            IsSeed:      true,
        },
        {
            WorkspaceID: workspaceID,
            Name:        "seed-deny-external",
            Description: "Block inbound traffic from outside the mesh",
            Action:      "DENY",
            PolicyTypes: `["Ingress"]`,
            Spec:        `{"selector":{},"ingress":[]}`,
            Status:      models.PolicyStatusActive,
            IsSeed:      true,
        },
    }
    for _, p := range policies {
        p.ID = uuid.NewString()
        if err := inj.store.Policies().Create(ctx, p); err != nil {
            return err
        }
    }
    return nil
}

func (inj *Injector) injectAlerts(ctx context.Context, workspaceID string) error {
    now := time.Now()
    ended := now.Add(-2 * time.Hour)

    alerts := []*models.AlertHistory{
        {
            Model:       models.Model{ID: uuid.NewString()},
            RuleID:      "seed-rule-offline",
            WorkspaceID: workspaceID,
            Status:      "resolved",
            Severity:    "warning",
            Labels:      `{"node":"node-shanghai-02"}`,
            Value:       0,
            Message:     "Node node-shanghai-02 went offline for 5 minutes",
            StartedAt:   now.Add(-3 * time.Hour),
            EndedAt:     &ended,
            Notified:    true,
            IsSeed:      true,
        },
        {
            Model:       models.Model{ID: uuid.NewString()},
            RuleID:      "seed-rule-latency",
            WorkspaceID: workspaceID,
            Status:      "resolved",
            Severity:    "info",
            Labels:      `{"node":"node-guangzhou-01"}`,
            Value:       280.5,
            Message:     "RTT to node-guangzhou-01 exceeded 250ms",
            StartedAt:   now.Add(-26 * time.Hour),
            EndedAt:     &ended,
            Notified:    true,
            IsSeed:      true,
        },
    }
    for _, a := range alerts {
        if err := inj.store.Alerts().CreateAlertHistory(ctx, a); err != nil {
            return err
        }
    }
    return nil
}
```

- [ ] **Step 4: 在相关 model 加 `IsSeed` 字段**

`internal/server/models/audit.go` 末尾的 `AuditLog` struct 加字段：
```go
IsSeed bool `gorm:"default:false;index" json:"isSeed,omitempty"`
```

`internal/server/models/policy.go` 的 `Policy` struct 加字段：
```go
IsSeed bool `gorm:"default:false;index" json:"isSeed,omitempty"`
```

`internal/server/models/alert.go` 的 `AlertHistory` struct 加字段：
```go
IsSeed bool `gorm:"default:false;index" json:"isSeed,omitempty"`
```

- [ ] **Step 5: 在 store 接口加 `Seed().Clear()` 方法**

查看 `internal/agent/store/` 目录中的 Store 接口定义文件，添加：

```go
// SeedRepository handles demo seed data lifecycle.
type SeedRepository interface {
    Clear(ctx context.Context, workspaceID string) error
}
```

并在 `Store` interface 中添加：
```go
Seed() SeedRepository
```

- [ ] **Step 6: 实现 gormstore 中的 SeedRepository**

新建 `internal/db/gormstore/seed.go`：

```go
// Copyright 2024 The Lattice Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gormstore

import (
    "context"

    "github.com/alatticeio/lattice/internal/server/models"
    "gorm.io/gorm"
)

type seedRepo struct{ db *gorm.DB }

func newSeedRepo(db *gorm.DB) *seedRepo { return &seedRepo{db: db} }

// Clear deletes all seed records belonging to workspaceID in one transaction.
func (r *seedRepo) Clear(ctx context.Context, workspaceID string) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := tx.Where("workspace_id = ? AND is_seed = ?", workspaceID, true).
            Delete(&models.AuditLog{}).Error; err != nil {
            return err
        }
        if err := tx.Where("workspace_id = ? AND is_seed = ?", workspaceID, true).
            Delete(&models.Policy{}).Error; err != nil {
            return err
        }
        return tx.Where("workspace_id = ? AND is_seed = ?", workspaceID, true).
            Delete(&models.AlertHistory{}).Error
    })
}
```

在 `internal/db/gormstore/store.go` 的 `GormStore` struct 添加字段：
```go
seed store.SeedRepository
```

在 `newStore()` 初始化中添加：
```go
seed: newSeedRepo(db),
```

添加 accessor：
```go
func (s *GormStore) Seed() store.SeedRepository { return s.seed }
```

- [ ] **Step 7: 运行测试**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/server/seed/... -v 2>&1 | tail -30
```

期望：`PASS`

- [ ] **Step 8: 提交**

```bash
git add internal/server/seed/ internal/server/models/audit.go internal/server/models/policy.go internal/server/models/alert.go internal/agent/store/ internal/db/gormstore/seed.go internal/db/gormstore/store.go
git commit -s -m "feat(playground): implement seed data injector with IsSeed tagging"
```

---

## Task 3：Workspace 创建时自动注入种子数据

**Files:**
- Modify: `internal/server/service/workspace.go`

- [ ] **Step 1: 在 `AddWorkspace` 中调用 InjectSeedData**

在 `internal/server/service/workspace.go` 的 `AddWorkspace` 方法中，紧接 workspace 创建成功后（`res = vo.WorkspaceVo{...}` 之前），添加：

```go
// Inject seed data for new workspace so Dashboard has content on first open.
injector := seed.NewInjector(s)
if seedErr := injector.Inject(ctx, newWs.ID, seed.DefaultOptions()); seedErr != nil {
    // Non-fatal: log and continue, workspace is usable without seed data.
    w.log.Warn("failed to inject seed data", "workspaceId", newWs.ID, "err", seedErr)
}
```

在文件顶部 import 块加入：
```go
"github.com/alatticeio/lattice/internal/server/seed"
```

- [ ] **Step 2: 运行现有 workspace 测试确认不回归**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/server/service/... -v 2>&1 | tail -20
```

期望：`PASS`（无已有测试失败）

- [ ] **Step 3: 提交**

```bash
git add internal/server/service/workspace.go
git commit -s -m "feat(playground): auto-inject seed data on new workspace creation"
```

---

## Task 4：暴露清除种子数据 API

**Files:**
- Modify: `internal/server/controller/workspace.go`
- Modify: `internal/server/server/workspace.go`

- [ ] **Step 1: 在 WorkspaceController 接口加 ClearSeedData**

`internal/server/controller/workspace.go` 的 `WorkspaceController` interface 加：
```go
ClearSeedData(ctx context.Context, workspaceID string) error
```

在 `workspaceController` struct 加字段：
```go
seedInjector *seed.Injector
```

实现方法：
```go
func (c *workspaceController) ClearSeedData(ctx context.Context, workspaceID string) error {
    return c.seedInjector.Clear(ctx, workspaceID)
}
```

修改 `NewWorkspaceController` 函数：
```go
func NewWorkspaceController(client *resource.Client, st store.Store) WorkspaceController {
    return &workspaceController{
        workspaceService: service.NewWorkspaceService(client, st),
        seedInjector:     seed.NewInjector(st),
    }
}
```

在 import 中加入：
```go
"github.com/alatticeio/lattice/internal/server/seed"
```

- [ ] **Step 2: 在 Gin 路由注册 DELETE 接口**

`internal/server/server/workspace.go` 的 `workspaceRouter()` 在 workspaceGroup 块内添加：

```go
workspaceGroup.DELETE("/:id/seed", s.middleware.AdminOnly(), s.handleClearSeed())
```

新增 handler：
```go
func (s *Server) handleClearSeed() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := c.Param("id")
        if id == "" {
            resp.BadRequest(c, "id is required")
            return
        }
        if err := s.workspaceController.ClearSeedData(c.Request.Context(), id); err != nil {
            resp.Error(c, err.Error())
            return
        }
        resp.OK(c, "seed data cleared")
    }
}
```

- [ ] **Step 3: 编译验证**

```bash
cd /Users/francis/workspc/lattice && go build ./... 2>&1
```

期望：无编译错误

- [ ] **Step 4: 提交**

```bash
git add internal/server/controller/workspace.go internal/server/server/workspace.go
git commit -s -m "feat(playground): add DELETE /workspaces/:id/seed API to clear seed data"
```

---

## Task 5：Demo 账号只读中间件

**Files:**
- Create: `internal/server/server/middleware/readonly.go`
- Create: `internal/server/server/middleware/readonly_test.go`

- [ ] **Step 1: 写失败测试**

创建 `internal/server/server/middleware/readonly_test.go`：

```go
// Copyright 2024 The Lattice Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package middleware_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/alatticeio/lattice/internal/server/server/middleware"
    "github.com/gin-gonic/gin"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestMiddleware(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Middleware Suite")
}

var _ = Describe("ReadOnly middleware", func() {
    var router *gin.Engine

    BeforeEach(func() {
        gin.SetMode(gin.TestMode)
        router = gin.New()
        router.Use(middleware.ReadOnly())
        router.POST("/api/v1/test", func(c *gin.Context) { c.Status(http.StatusOK) })
        router.GET("/api/v1/test", func(c *gin.Context) { c.Status(http.StatusOK) })
    })

    It("blocks POST requests with 403", func() {
        w := httptest.NewRecorder()
        req, _ := http.NewRequest(http.MethodPost, "/api/v1/test", nil)
        router.ServeHTTP(w, req)
        Expect(w.Code).To(Equal(http.StatusForbidden))
    })

    It("allows GET requests", func() {
        w := httptest.NewRecorder()
        req, _ := http.NewRequest(http.MethodGet, "/api/v1/test", nil)
        router.ServeHTTP(w, req)
        Expect(w.Code).To(Equal(http.StatusOK))
    })

    It("blocks PUT requests with 403", func() {
        w := httptest.NewRecorder()
        req, _ := http.NewRequest(http.MethodPut, "/api/v1/test", nil)
        router.ServeHTTP(w, req)
        Expect(w.Code).To(Equal(http.StatusForbidden))
    })

    It("blocks DELETE requests with 403", func() {
        w := httptest.NewRecorder()
        req, _ := http.NewRequest(http.MethodDelete, "/api/v1/test", nil)
        router.ServeHTTP(w, req)
        Expect(w.Code).To(Equal(http.StatusForbidden))
    })
})
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/server/server/middleware/... -v 2>&1 | tail -20
```

期望：`FAIL` — `cannot find package middleware.ReadOnly`

- [ ] **Step 3: 实现 ReadOnly 中间件**

创建 `internal/server/server/middleware/readonly.go`：

```go
// Copyright 2024 The Lattice Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package middleware

import (
    "net/http"

    "github.com/alatticeio/lattice/pkg/utils/resp"
    "github.com/gin-gonic/gin"
)

// ReadOnly blocks all non-GET/HEAD/OPTIONS requests.
// Used for the demo account to prevent visitors from mutating shared state.
func ReadOnly() gin.HandlerFunc {
    return func(c *gin.Context) {
        switch c.Request.Method {
        case http.MethodGet, http.MethodHead, http.MethodOptions:
            c.Next()
        default:
            resp.Forbidden(c, "demo account is read-only")
            c.Abort()
        }
    }
}
```

注意：检查 `pkg/utils/resp` 包中是否有 `Forbidden` 函数。如果没有，先添加：

```go
// 在 pkg/utils/resp/resp.go 中添加
func Forbidden(c *gin.Context, msg string) {
    c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": msg})
}
```

- [ ] **Step 4: 运行测试，确认通过**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/server/server/middleware/... -v 2>&1 | tail -20
```

期望：`PASS`

- [ ] **Step 5: 提交**

```bash
git add internal/server/server/middleware/readonly.go internal/server/server/middleware/readonly_test.go
git commit -s -m "feat(playground): add ReadOnly middleware for demo account"
```

---

## Task 6：安装脚本

**Files:**
- Create: `scripts/install.sh`

- [ ] **Step 1: 创建安装脚本**

创建 `scripts/install.sh`：

```bash
#!/usr/bin/env sh
# Lattice Agent Quick Install Script
# Usage: curl -sSL https://get.lattice.io | sh -s -- --token <TOKEN> --name <NODE_NAME>
#
# Copyright 2024 The Lattice Authors.
# Licensed under the Apache License, Version 2.0

set -e

LATTICE_VERSION="${LATTICE_VERSION:-latest}"
CDN_BASE="${CDN_BASE:-https://dl.lattice.io}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── parse args ───────────────────────────────────────────────────────────────
TOKEN=""
NODE_NAME=""

while [ $# -gt 0 ]; do
  case "$1" in
    --token)  TOKEN="$2";     shift 2 ;;
    --name)   NODE_NAME="$2"; shift 2 ;;
    *)        echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [ -z "$TOKEN" ] || [ -z "$NODE_NAME" ]; then
  echo "Usage: install.sh --token <WORKSPACE_TOKEN> --name <NODE_NAME>"
  exit 1
fi

# ── detect OS / arch ─────────────────────────────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# ── resolve version ──────────────────────────────────────────────────────────
if [ "$LATTICE_VERSION" = "latest" ]; then
  LATTICE_VERSION="$(curl -fsSL "${CDN_BASE}/stable.txt")"
fi

BINARY_URL="${CDN_BASE}/releases/${LATTICE_VERSION}/lattice-agent-${OS}-${ARCH}"
echo "Downloading Lattice Agent ${LATTICE_VERSION} for ${OS}/${ARCH}..."
curl -fsSL -o /tmp/lattice-agent "$BINARY_URL"
chmod +x /tmp/lattice-agent
mv /tmp/lattice-agent "${INSTALL_DIR}/lattice-agent"

echo "Installed to ${INSTALL_DIR}/lattice-agent"

# ── start agent ──────────────────────────────────────────────────────────────
if [ "$OS" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
  cat > /etc/systemd/system/lattice-agent.service <<EOF
[Unit]
Description=Lattice Agent
After=network.target

[Service]
ExecStart=${INSTALL_DIR}/lattice-agent --token ${TOKEN} --name ${NODE_NAME}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now lattice-agent
  echo "Lattice Agent started via systemd. Run: journalctl -u lattice-agent -f"
elif [ "$OS" = "darwin" ]; then
  cat > ~/Library/LaunchAgents/io.lattice.agent.plist <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>      <string>io.lattice.agent</string>
  <key>ProgramArguments</key>
  <array>
    <string>${INSTALL_DIR}/lattice-agent</string>
    <string>--token</string>  <string>${TOKEN}</string>
    <string>--name</string>   <string>${NODE_NAME}</string>
  </array>
  <key>RunAtLoad</key>  <true/>
  <key>KeepAlive</key>  <true/>
</dict>
</plist>
EOF
  launchctl load ~/Library/LaunchAgents/io.lattice.agent.plist
  echo "Lattice Agent started via launchd."
else
  echo "Run manually: ${INSTALL_DIR}/lattice-agent --token ${TOKEN} --name ${NODE_NAME}"
fi

echo "Done. Your node '${NODE_NAME}' should appear in the Dashboard within 30 seconds."
```

- [ ] **Step 2: 使脚本可执行并 lint**

```bash
chmod +x /Users/francis/workspc/lattice/scripts/install.sh
sh -n /Users/francis/workspc/lattice/scripts/install.sh
```

期望：无语法错误输出

- [ ] **Step 3: 提交**

```bash
git add scripts/install.sh
git commit -s -m "feat(playground): add quick install script for Lattice Agent"
```

---

## Task 7：Sealos 一键部署模板

**Files:**
- Create: `deploy/sealos/app.yaml`
- Create: `deploy/demo/docker-compose.yml`

- [ ] **Step 1: 创建 Sealos 模板**

创建 `deploy/sealos/app.yaml`：

```yaml
# Copyright 2024 The Lattice Authors. Apache 2.0 License.
apiVersion: app.sealos.io/v1
kind: Template
metadata:
  name: lattice
  labels:
    app.sealos.io/category: "networking"
spec:
  title: "Lattice"
  url: "https://lattice.io"
  gitRepo: "https://github.com/alatticeio/lattice"
  author: "Lattice Authors"
  description: "Self-hosted WireGuard Mesh Orchestration. Kubernetes-native, AI-powered, open-core."
  readme: "https://raw.githubusercontent.com/alatticeio/lattice/master/README.md"
  icon: "https://lattice.io/logo.svg"
  templateType: inline
  defaults:
    app_host:
      type: random
      value: ${{ random(8) }}
    app_name:
      type: random
      value: lattice-${{ random(8) }}
  inputs:
    admin_email:
      description: "Admin email address"
      type: string
      default: "admin@example.com"
      required: true
    admin_password:
      description: "Admin password (min 8 characters)"
      type: string
      default: ""
      required: true

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${{ defaults.app_name }}
  labels:
    app: ${{ defaults.app_name }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${{ defaults.app_name }}
  template:
    metadata:
      labels:
        app: ${{ defaults.app_name }}
    spec:
      containers:
        - name: latticed
          image: ghcr.io/alatticeio/latticed:latest
          ports:
            - containerPort: 8080
            - containerPort: 4222
          env:
            - name: LATTICE_ADMIN_EMAIL
              value: ${{ inputs.admin_email }}
            - name: LATTICE_ADMIN_PASSWORD
              value: ${{ inputs.admin_password }}
            - name: LATTICE_SEED_DATA
              value: "true"
          resources:
            requests:
              cpu: "250m"
              memory: "256Mi"
            limits:
              cpu: "1"
              memory: "512Mi"

---
apiVersion: v1
kind: Service
metadata:
  name: ${{ defaults.app_name }}
spec:
  selector:
    app: ${{ defaults.app_name }}
  ports:
    - name: http
      port: 8080
      targetPort: 8080

---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ${{ defaults.app_name }}
  annotations:
    kubernetes.io/ingress.class: nginx
spec:
  rules:
    - host: ${{ defaults.app_host }}.${{ SEALOS_CLOUD_DOMAIN }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: ${{ defaults.app_name }}
                port:
                  number: 8080
```

- [ ] **Step 2: 创建 Demo 环境 docker-compose**

创建 `deploy/demo/docker-compose.yml`：

```yaml
# Demo environment for demo.lattice.io
# Runs latticed in read-only API mode with pre-seeded data.
# Copyright 2024 The Lattice Authors. Apache 2.0 License.
version: "3.9"

services:
  latticed:
    image: ghcr.io/alatticeio/latticed:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "4222:4222"
    environment:
      LATTICE_MODE: demo           # enables read-only API + auto-login
      LATTICE_SEED_DATA: "true"
      LATTICE_DEMO_RESET_CRON: "0 0 * * *"   # reset daily at midnight
    volumes:
      - lattice-data:/data

  # Lightweight traffic generator: keeps demo nodes "active"
  demo-agent-1:
    image: ghcr.io/alatticeio/lattice-agent:latest
    restart: unless-stopped
    depends_on: [latticed]
    environment:
      LATTICE_SERVER: "http://latticed:8080"
      LATTICE_TOKEN: "${DEMO_TOKEN}"
      LATTICE_NODE_NAME: "node-beijing-01"
    cap_add: [NET_ADMIN]

  demo-agent-2:
    image: ghcr.io/alatticeio/lattice-agent:latest
    restart: unless-stopped
    depends_on: [latticed]
    environment:
      LATTICE_SERVER: "http://latticed:8080"
      LATTICE_TOKEN: "${DEMO_TOKEN}"
      LATTICE_NODE_NAME: "node-shanghai-01"
    cap_add: [NET_ADMIN]

  demo-agent-3:
    image: ghcr.io/alatticeio/lattice-agent:latest
    restart: unless-stopped
    depends_on: [latticed]
    environment:
      LATTICE_SERVER: "http://latticed:8080"
      LATTICE_TOKEN: "${DEMO_TOKEN}"
      LATTICE_NODE_NAME: "node-guangzhou-01"
    cap_add: [NET_ADMIN]

volumes:
  lattice-data:
```

- [ ] **Step 3: 提交**

```bash
git add deploy/sealos/app.yaml deploy/demo/docker-compose.yml
git commit -s -m "feat(playground): add Sealos template and demo docker-compose"
```

---

## Task 8：运行全量测试确认无回归

- [ ] **Step 1: 运行所有单元测试**

```bash
cd /Users/francis/workspc/lattice && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```

期望：无 `FAIL`

- [ ] **Step 2: 运行 lint**

```bash
cd /Users/francis/workspc/lattice && make lint 2>&1 | tail -20
```

期望：无 error（warnings 可接受）

- [ ] **Step 3: 提交最终状态**

```bash
git add -A
git status
```

确认只有预期文件变更，无意外文件。

---

## 交付检查清单

- [ ] `is_seed=true` 字段在 AuditLog、Policy、AlertHistory 三个表
- [ ] 新 Workspace 创建后 Dashboard 有数据（8 节点历史 + 20 条审计 + 3 条策略 + 2 条告警）
- [ ] `DELETE /api/v1/workspaces/:id/seed` 接口可清除种子数据
- [ ] ReadOnly 中间件阻断 POST/PUT/DELETE，返回 403
- [ ] `scripts/install.sh` 支持 Linux amd64/arm64 和 macOS
- [ ] `deploy/sealos/app.yaml` 包含完整 Deployment + Service + Ingress
- [ ] `deploy/demo/docker-compose.yml` 包含 latticed + 3 个 agent 节点
- [ ] 全量测试无 FAIL
