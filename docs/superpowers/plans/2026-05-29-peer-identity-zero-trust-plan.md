# PeerIdentity 实现计划

**日期**：2026-05-29  
**Spec**：[2026-05-29-peer-identity-zero-trust-design.md](../specs/2026-05-29-peer-identity-zero-trust-design.md)  
**状态**：Ready

---

## 总体时间估算

| 阶段 | 工作量 |
|------|--------|
| CRD 定义 | 1 天 |
| Controller | 2 天 |
| Server + DB | 2 天 |
| 反向索引 | 0.5 天 |
| 测试 | 1.5 天 |
| **合计** | **7 天** |

---

## 第一阶段：CRD 定义（1 天）

### 步骤 1.1：新增 PeerIdentity 类型

**文件**：`api/v1alpha1/peer_identity_types.go`

- 定义 `PeerIdentitySpec`（Network, PeerRef, PreviousPeerRef, GracePeriodSeconds, Description）
- 定义 `PeerIdentityStatus`（ResolvedPeerIP, PreviousPeerIP, GracePeriodExpiresAt, Conditions）
- 定义 `PeerIdentity` 和 `PeerIdentityList` 结构体
- 注册到 `SchemeBuilder`

### 步骤 1.2：扩展 PeerSelection

**文件**：`api/v1alpha1/lattice_policy_types.go`

```go
type PeerSelection struct {
    PeerSelector *metav1.LabelSelector `json:"peerSelector,omitempty"`
    IPBlock      *IPBlock              `json:"ipBlock,omitempty"`
    IdentityRef  *string               `json:"identityRef,omitempty"` // 新增
}
```

### 步骤 1.3：生成代码

```bash
make generate    # 生成 deepcopy
make manifests   # 生成 CRD YAML
```

验证：`config/crd/bases/` 下出现 `alattice.io_peeridentities.yaml`

---

## 第二阶段：Controller（2 天）

### 步骤 2.1：PeerIdentity Controller

**文件**：`internal/agent/controller/peer_identity_controller.go`

```
Reconcile(ctx, req) {
    1. 获取 PeerIdentity 实例
    2. 根据 spec.peerRef 查找 LatticePeer
    3. 从 LatticePeer.status.allocatedAddress 取 IP
    4. 写入 PeerIdentity.status.resolvedPeerIP
    5. 若 spec.previousPeerRef 非空：
       a. 查找旧 LatticePeer，取旧 IP
       b. 写入 status.previousPeerIP
       c. 计算 gracePeriodExpiresAt = now + gracePeriodSeconds
    6. 回写 LatticePeer.status.identityRef = PeerIdentity.name
    7. Enqueue 所有引用此 PeerIdentity 的 LatticePolicy（触发 policy reconcile）
}
```

**Watch 设置**：
- 主资源：`PeerIdentity`
- 次级资源：`LatticePeer`（当 LatticePeer.status.allocatedAddress 变化时，enqueue 关联的 PeerIdentity）

### 步骤 2.2：GracePeriod Controller

**文件**：复用 `internal/agent/controller/policy_ttl_controller.go` 模式

```
Reconcile(ctx, req) {
    1. List 所有 PeerIdentity
    2. 筛选 gracePeriodExpiresAt < now 且 previousPeerRef 非空的
    3. 清空 previousPeerRef、previousPeerIP、gracePeriodExpiresAt
    4. 触发关联 LatticePolicy reconcile
}
```

### 步骤 2.3：扩展 PolicyEvaluator

**文件**：`internal/agent/controller/policy_evaluator.go`

在 `resolveRulePeers()` 中新增分支：

```go
if rule.IdentityRef != nil {
    // 查 PeerIdentity (network, name)
    // 收集 resolvedPeerIP
    // 若 previousPeerIP 存在且宽限期未过期，一并收集
    // 追加到 peers 列表
}
```

现有 `peerSelector` / `ipBlock` 逻辑不变，三种方式取并集。

---

## 第三阶段：Server + DB 层（2 天）

### 步骤 3.1：DB Model

**文件**：`internal/server/models/peer_identity.go`

```go
type PeerIdentity struct {
    Model
    NetworkID           string     `gorm:"size:36;uniqueIndex:idx_pi_network_name"`
    Name                string     `gorm:"size:200;uniqueIndex:idx_pi_network_name"`
    PeerRef             string     `gorm:"size:200"`
    PreviousPeerRef     string     `gorm:"size:200"`
    GracePeriodSeconds  int        `gorm:"default:300"`
    ResolvedPeerIP      string     `gorm:"size:64"`
    PreviousPeerIP      string     `gorm:"size:64"`
    GracePeriodExpiresAt *time.Time
    Description         string     `gorm:"size:500"`
}
```

**文件**：`internal/db/gormstore/peer_identity.go`

- AutoMigrate
- CRUD 方法（GetByName, List, Create, Update, Delete）

### 步骤 3.2：Service 层

**文件**：`internal/server/service/peer_identity.go`

```
GetPeerIdentity(networkID, name) → PeerIdentity
ListPeerIdentities(networkID) → []PeerIdentity
CreatePeerIdentity(input) → PeerIdentity
UpdatePeerIdentity(id, input) → PeerIdentity
DeletePeerIdentity(id) error
```

### 步骤 3.3：API Router

**文件**：`internal/server/server/peer_identity.go`

```
GET    /api/v1/networks/:networkId/peer-identities
GET    /api/v1/networks/:networkId/peer-identities/:name
POST   /api/v1/networks/:networkId/peer-identities
PUT    /api/v1/networks/:networkId/peer-identities/:name
DELETE /api/v1/networks/:networkId/peer-identities/:name
```

### 步骤 3.4：注册/心跳携带 identityName

**文件**：`internal/server/service/peer.go` 或注册相关 handler

在注册响应 DTO 中新增 `IdentityName string` 字段，从 `t_peer_identity` 查询填充。

---

## 第四阶段：LatticePeer 反向索引（0.5 天）

### 步骤 4.1：扩展 LatticePeerStatus

**文件**：`api/v1alpha1/lattice_peer_types.go`

```go
type LatticePeerStatus struct {
    // ... 现有字段 ...
    IdentityRef string `json:"identityRef,omitempty"` // 新增
}
```

### 步骤 4.2：Controller 回写

在 PeerIdentity Controller 的 Reconcile 步骤 6 中实现：

```go
peer.Status.IdentityRef = peerIdentity.Name
// 更新 LatticePeer CRD / DB
```

---

## 第五阶段：测试（1.5 天）

### 步骤 5.1：单元测试

- `policy_evaluator_test.go`：identityRef 解析分支
- `peer_identity_controller_test.go`：reconcile 逻辑
- `peer_identity_service_test.go`：CRUD + 查询

### 步骤 5.2：E2E 测试

**文件**：`test/e2e/peer_identity_test.go`

```
场景 1：创建 PeerIdentity，验证 policy 自动解析到正确 IP
场景 2：设备替换（peerRef 变更），验证宽限期双 IP 生效
场景 3：宽限期过期，验证旧 IP 自动移除
场景 4：删除 PeerIdentity，验证关联 policy 立即失效
```

---

## 依赖关系

```
CRD 定义 (1.1, 1.2)
    ↓
Controller (2.1, 2.2, 2.3) ──→ 测试 (5.1, 5.2)
    ↓
Server + DB (3.1, 3.2, 3.3, 3.4)
    ↓
反向索引 (4.1)
```

Controller 和 Server + DB 可以并行开发，但测试需要两者都完成。
