# Wireflow 用户身份与权限系统设计

> 版本：v1.0 | 日期：2026-04-22

---

## 1. 目标

| 目标 | 说明 |
|------|------|
| **外部数据源接入** | 支持 LDAP/AD、企业 OIDC、GitHub 等外部身份源，通过 Dex 统一联邦，Wireflow 不自建协议实现 |
| **邀请制成员管理** | 管理员可邀请外部用户加入工作区，支持邮件/链接方式 |
| **K8s RBAC 作为权限后端** | 资源操作权限完全由 K8s RoleBinding 控制，管理 API 通过 impersonation 触发原生鉴权，不维护自定义权限表 |

---

## 2. 整体架构

```
外部身份源
┌─────────────┐
│  LDAP / AD  │──┐
│  GitHub     │──┤    ┌─────────────────┐    ┌─────────────────────┐
│  企业 OIDC  │──┼───>│   Dex（联邦层）  │───>│  Wireflow 管理 API  │
│  SAML       │──┘    │  ID Token 签发   │    │  JWT 签发 / 续签     │
└─────────────┘        └─────────────────┘    └─────────┬───────────┘
                                                         │ impersonate
本地账号 ────────────────────────────────────────────────┘
                                                         ▼
                                               ┌─────────────────────┐
                                               │   Kubernetes API    │
                                               │  RoleBinding 鉴权   │
                                               └─────────────────────┘
```

### 分层说明

| 层 | 职责 | 实现位置 |
|----|------|---------|
| **身份联邦层** | 统一认证入口，代理 LDAP/OIDC/SAML | Dex（已有） |
| **平台身份层** | User + UserIdentity，管理平台内用户档案 | DB：`t_user` + `t_user_identity` |
| **成员关系层** | 工作区内的角色绑定和邀请流 | DB：`t_workspace_member` + `t_workspace_invitation` |
| **资源权限层** | 谁能对哪个 CRD 做什么操作 | K8s RoleBinding（已有框架） |

---

## 3. 数据模型

### 3.1 用户身份模型

#### `t_user` — 平台用户（规范身份）

```go
type User struct {
    Model                                          // ID (UUID), CreatedAt, UpdatedAt, DeletedAt
    SystemRole  SystemRole `gorm:"type:varchar(20)"` // platform_admin | user
    DisplayName string
    Email       string     `gorm:"uniqueIndex"`      // 主 email，用于通知和邀请匹配
    Avatar      string
    UserProfile *UserProfile
    Identities  []UserIdentity                      // 1:N，关联所有外部账号
}

type SystemRole string
const (
    SystemRolePlatformAdmin SystemRole = "platform_admin"
    SystemRoleUser          SystemRole = "user"
)
```

> **删除字段**：`User.ExternalID`（移入 UserIdentity）、`User.Workspaces []Workspace`（删除 many2many tag，改用 WorkspaceMember 直接查询）

#### `t_user_identity` — 外部身份链接（新增）

```go
type UserIdentity struct {
    Model
    UserID     string    `gorm:"index;not null"`    // FK → t_user.id
    Provider   string    `gorm:"size:50;not null"`  // "local" | "dex" | "ldap" | "github" | "oidc"
    ExternalID string    `gorm:"not null"`           // IdP 侧的 subject
    Email      string                                // IdP 返回的 email（可能和主 email 不同）
    Metadata   string    `gorm:"type:text"`          // JSON: name, groups, raw claims
    LastSyncAt time.Time
}
// 唯一约束：(Provider, ExternalID)
```

**作用：**
- 同一个 User 可以有多个 identity（本地账号 + GitHub + 企业 SSO）
- 首次 SSO 登录时：按 email 查找已有 User，找到则关联，找不到则创建新 User

### 3.2 工作区成员模型

#### `t_workspace_member` — 成员关系（调整）

```go
type WorkspaceMember struct {
    Model
    WorkspaceID string            `gorm:"index;not null"`
    UserID      string            `gorm:"index;not null"`
    Role        WorkspaceRole     `gorm:"type:varchar(20);not null"`  // admin|editor|member|viewer
    Status      string            `gorm:"type:varchar(20);default:'active'"` // active|pending
    InvitedBy   string            // FK → t_user.id，邀请人
    JoinedAt    *time.Time        // 接受邀请时间
}
// 唯一约束：(WorkspaceID, UserID)
```

> **删除**：`User` 和 `Workspace` 关联字段（避免双重管理同一张表）

#### `t_workspace_invitation` — 邀请记录（新增）

```go
type WorkspaceInvitation struct {
    Model
    WorkspaceID string        `gorm:"index;not null"`
    InviterID   string        // FK → t_user.id
    Email       string        // 被邀请人邮箱（此时可能尚无账号）
    Role        WorkspaceRole
    Token       string        `gorm:"uniqueIndex"` // 安全随机 token（32 字节 hex）
    Status      string        // pending | accepted | expired | revoked
    ExpiresAt   time.Time
}
```

### 3.3 删除的冗余模型

| 模型 | 原因 |
|------|------|
| `RoleAssignment` | 被 `WorkspaceMember.Role` + `User.SystemRole` 替代，K8s RBAC 替代 namespace 权限 |
| `UserNamespacePermission` | 同上，由 K8s RoleBinding 接管 |
| `Namespace`（models） | K8s namespace 直接从 Workspace.Namespace 派生，无需独立表 |

---

## 4. K8s RBAC 设计

### 4.1 ClusterRole 定义

工作区初始化时（已有 `InitializeTenant`）创建以下 ClusterRole，**整个集群只需定义一次**：

```yaml
# wireflow-admin
rules:
- apiGroups: ["wireflowcontroller.wireflow.run"]
  resources: ["wireflowpeers", "wireflownetworks", "wireflowpolicies",
              "wireflowenrollmenttokens", "wireflowrelayservers"]
  verbs: ["*"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch", "create", "delete"]

# wireflow-editor
rules:
- apiGroups: ["wireflowcontroller.wireflow.run"]
  resources: ["wireflowpeers", "wireflownetworks"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["wireflowcontroller.wireflow.run"]
  resources: ["wireflowpolicies", "wireflowenrollmenttokens"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]

# wireflow-member
rules:
- apiGroups: ["wireflowcontroller.wireflow.run"]
  resources: ["wireflowpeers"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: ["wireflowcontroller.wireflow.run"]
  resources: ["wireflownetworks", "wireflowpolicies"]
  verbs: ["get", "list", "watch"]

# wireflow-viewer
rules:
- apiGroups: ["wireflowcontroller.wireflow.run"]
  resources: ["wireflowpeers", "wireflownetworks", "wireflowpolicies"]
  verbs: ["get", "list", "watch"]
```

### 4.2 RoleBinding 策略

工作区创建时（`InitializeTenant`，已有实现）为每个角色创建 RoleBinding，绑定到 K8s Group：

```
namespace: wf-{workspaceID}

RoleBinding: wf-rb-{wsID}-admin
  subjects: [{kind: Group, name: "wf-group-{wsID}-admin"}]
  roleRef:   ClusterRole/wireflow-admin

RoleBinding: wf-rb-{wsID}-editor
  subjects: [{kind: Group, name: "wf-group-{wsID}-editor"}]
  roleRef:   ClusterRole/wireflow-editor

RoleBinding: wf-rb-{wsID}-member
  subjects: [{kind: Group, name: "wf-group-{wsID}-member"}]
  roleRef:   ClusterRole/wireflow-member

RoleBinding: wf-rb-{wsID}-viewer
  subjects: [{kind: Group, name: "wf-group-{wsID}-viewer"}]
  roleRef:   ClusterRole/wireflow-viewer
```

> 当前代码只创建了 `admin`/`member`/`viewer` 三种，需补充 `editor`。

### 4.3 管理 API Impersonation 流程

管理 API 对 K8s 资源的操作通过 **impersonation** 让 K8s 原生鉴权：

```
用户请求: DELETE /api/v1/peers/:peerID
         Headers: Authorization: Bearer <jwt>
                  X-Workspace-Id: <wsID>
         │
         ▼
AuthMiddleware: 解析 JWT → userID, systemRole
         │
         ▼
TenantMiddleware: 加载 WorkspaceMember → role = "member"
         │
         ▼
PeerController:
  k8sClient.WithImpersonation({
      UserName: "wf-user-{userID}",
      Groups:   ["wf-group-{wsID}-member"],
  }).Delete(peer)
         │
         ▼
K8s API Server: 查 RoleBinding → wireflow-member 无 delete 权限 → 403
         │
         ▼
管理 API 返回: 403 Forbidden
```

**实现**：使用 `controller-runtime` 的 `client.WithOptions(client.ImpersonateUser(...))` 或原生 `k8s.io/client-go` 的 `ImpersonationConfig`。`IdentityImpersonator`（已有）就是为此设计的。

### 4.4 管理 API 层的角色校验（轻量保留）

对于纯管理平面操作（邀请成员、修改工作区配置等，不涉及 CRD 资源），仍使用数据库角色校验，**不走 K8s**：

```
WorkspaceAuthMiddleware(requiredRole=admin):
  查 WorkspaceMember.Role → weight check → pass/deny
```

两层的边界：

| 操作类型 | 权限后端 |
|----------|---------|
| WireflowPeer / Network / Policy / Token 增删改查 | K8s RBAC（impersonation） |
| 邀请成员、修改成员角色、删除工作区 | DB WorkspaceMember 角色校验 |
| 平台级操作（创建工作区、管理用户） | `User.SystemRole == platform_admin` |

---

## 5. 外部数据源接入

### 5.1 Dex 作为联邦枢纽

Wireflow 自身**不实现** LDAP、SAML 或其他 IdP 协议。所有外部身份源统一接入 Dex，Wireflow 只作为一个 OIDC 客户端：

```
管理员配置 Dex connector：
┌─────────────────────────────────────┐
│ Dex connectors:                     │
│  - id: ldap                         │
│    type: ldap                       │
│    config: {host, bindDN, ...}      │
│  - id: github                       │
│    type: github                     │
│    config: {clientID, clientSecret} │
│  - id: oidc-corp                    │
│    type: oidc                       │
│    config: {issuer, ...}            │
└─────────────────────────────────────┘
         │
         ▼ ID Token (sub, email, groups)
┌─────────────────────┐
│ Wireflow /auth/callback │  ← 已有实现
│  OnboardExternalUser()  │  ← 改造为 UserIdentity 模型
└─────────────────────┘
```

**优点**：
- Wireflow 代码零改动即可支持新 IdP（只需配置 Dex）
- 企业 LDAP、GitHub、Google 等全部复用 Dex 的成熟实现
- Dex 处理 token 刷新、会话管理

### 5.2 OnboardExternalUser 改造

当前实现（单 ExternalID）→ 改为 UserIdentity 关联：

```go
func (s *userService) OnboardExternalUser(ctx context.Context, provider, externalID, email string) (*models.User, error) {
    // 1. 查找已有 identity
    identity, err := s.store.UserIdentities().GetByProviderAndExternalID(ctx, provider, externalID)
    if err == nil {
        return s.store.Users().GetByID(ctx, identity.UserID)
    }

    // 2. 按 email 查找已有 user（账号合并）
    user, err := s.store.Users().GetByEmail(ctx, email)
    if errors.Is(err, gorm.ErrRecordNotFound) {
        // 3. 创建新 user
        user = &models.User{Email: email, SystemRole: models.SystemRoleUser}
        if err := s.store.Users().Create(ctx, user); err != nil {
            return nil, err
        }
    }

    // 4. 关联新 identity
    identity = &models.UserIdentity{
        UserID:     user.ID,
        Provider:   provider,
        ExternalID: externalID,
        Email:      email,
        LastSyncAt: time.Now(),
    }
    s.store.UserIdentities().Create(ctx, identity)
    return user, nil
}
```

### 5.3 SCIM 2.0（可选，企业级）

SCIM 用于企业 IdP（如 Okta、Azure AD）主动推送用户变更（创建/禁用/删除），无需用户首次登录才入库。

实现路径：新增 `/scim/v2` 路由，实现 `Users` 和 `Groups` endpoint，写入 `UserIdentity` 表。这是独立模块，不影响现有认证流程，**按需实现**。

---

## 6. 邀请流设计

```
管理员                    系统                       被邀请人
   │                        │                            │
   │  POST /workspaces/:id/invitations                   │
   │  {email, role}         │                            │
   │──────────────────────>│                            │
   │                        │ 生成 token                 │
   │                        │ 写 t_workspace_invitation  │
   │                        │ 发送邀请邮件 ──────────────>│
   │                        │                            │
   │                        │        GET /invite/{token} │
   │                        │<───────────────────────────│
   │                        │ 验证 token 有效期           │
   │                        │ 若未登录 → 跳转登录页        │
   │                        │ 登录后 Accept               │
   │                        │                            │
   │                        │  POST /invite/{token}/accept│
   │                        │<───────────────────────────│
   │                        │ 写 WorkspaceMember          │
   │                        │ 更新 invitation.Status=accepted│
   │                        │ 返回工作区跳转 URL ──────────>│
```

### 关键规则

- token 有效期：7 天
- 同一 email 在同一工作区只能有一条 pending 邀请
- 被邀请人不存在账号时：完成注册/SSO 后自动 accept（email 匹配）
- 邀请方角色上限：不能邀请高于自己角色的成员

---

## 7. JWT 设计调整

**当前问题**：`WireFlowClaims.WorkspaceId` 意义模糊，和 `X-Workspace-Id` header 重复。

**调整方案**：JWT 只携带平台级身份，工作区上下文完全由 header 传递：

```go
type WireFlowClaims struct {
    jwt.RegisteredClaims               // sub=userID, exp, iat, iss
    Email      string `json:"email"`
    SystemRole string `json:"system_role"` // platform_admin | user
    // 删除 WorkspaceId 字段
}
```

- 登录/SSO 回调：签发 JWT（24h）
- 切换工作区：前端切换 `X-Workspace-Id` header，无需重新签发 token
- 需要续签：refresh token 或重新登录（可选实现）

---

## 8. 现有代码的改动清单

### 必须改（影响正确性）

| 文件 | 改动 |
|------|------|
| `models/user.go` | `Role` → `SystemRole SystemRole`；删除 `many2many` tag；删除 `ExternalID`；添加 `UserIdentity` model |
| `models/workspace.go` | 删除 `Members []User` 的 `many2many` tag；`WorkspaceMember` 添加 `InvitedBy`、`JoinedAt`；添加 `WorkspaceInvitation` |
| `service/workspace.go` | `WorkspaceMemberService.Create/Update/Delete` 实现（当前 panic）；`OnboardExternalUser` 改为 UserIdentity 模型 |
| `service/workspace.go` | `InitializeTenant` 补充 `editor` RoleBinding |
| `middleware/auth.go` | JWT claims 中 role 字段改为 `system_role`，删除 workspace role 硬编码 |
| `middleware/tenant.go` | 补充 `TenantContextMiddleware` 里缺失的成员校验（当前有注释 TODO） |

### 建议改（提升设计一致性）

| 文件 | 改动 |
|------|------|
| `models/role.go` | 删除整个文件（`RoleAssignment` 废弃） |
| `models/user.go` | 删除 `Namespace`、`UserNamespacePermission`（废弃） |
| `pkg/utils/jwt.go` | 删除 `WorkspaceId`，添加 `SystemRole` 字段 |
| `db/gormstore/migrate.go` | 新增 `UserIdentity`、`WorkspaceInvitation` 迁移；移除废弃表迁移 |

### 按需实现（新功能）

| 功能 | 文件 |
|------|------|
| 邀请流 API | `management/server/invitation.go`、`service/invitation.go` |
| UserIdentity 仓库 | `internal/store/store.go`、`internal/db/gormstore/identity.go` |
| K8s Impersonation 封装 | `management/resource/impersonator.go`（`IdentityImpersonator` 已有占位） |
| SCIM 2.0（可选） | `management/server/scim/` |

---

## 9. 数据库迁移路径

```sql
-- 新增
CREATE TABLE t_user_identity (...);
CREATE TABLE t_workspace_invitation (...);

-- 存量数据迁移：ExternalID → UserIdentity
INSERT INTO t_user_identity (user_id, provider, external_id, email, last_sync_at)
SELECT id, 'dex', external_id, email, NOW()
FROM t_user WHERE external_id != '';

-- 清理冗余列（可延后）
ALTER TABLE t_user DROP COLUMN external_id;
ALTER TABLE t_user RENAME COLUMN role TO system_role;

-- 删除废弃表（可延后）
DROP TABLE IF EXISTS t_role_assignment;
DROP TABLE IF EXISTS t_user_namespace_permission;
DROP TABLE IF EXISTS t_namespace;
```

---

## 10. 总结

| 维度 | 现状 | 改造后 |
|------|------|--------|
| 外部用户源 | 单 ExternalID，只支持一个 IdP | UserIdentity 表，N 个 IdP，Dex 统一联邦 |
| 邀请机制 | Status 字段存在，逻辑空实现 | WorkspaceInvitation 完整流程 |
| 资源权限 | 管理 API 自建角色校验 | K8s RoleBinding + impersonation，无自定义 RBAC |
| 角色模型 | 全局/工作区共用 WorkspaceRole 类型 | SystemRole（平台级） + WorkspaceRole（工作区级） 分离 |
| JWT | 携带 WorkspaceId，语义模糊 | 只携带平台身份，工作区由 header 传递 |
