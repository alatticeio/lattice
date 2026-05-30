# PeerIdentity：基于身份的 Zero Trust 策略层设计

> **注意**：本文档已合并至 [AI Agent Zero Trust 安全网络设计](./2026-05-30-ai-agent-zero-trust-network-design.md)，请参考最新版本。

**日期**：2026-05-29  
**状态**：Deprecated（已合并）  
**最新版本**：[AI Agent Zero Trust 安全网络设计](./2026-05-30-ai-agent-zero-trust-network-design.md)  
**关联文档**：[AI Agent Secure Mesh](./2026-05-29-ai-agent-secure-mesh-design.md)、[Agent Sandbox](./2026-05-29-agent-sandbox-design.md)

---

## 背景与动机

### 现状问题

Lattice 现有 `LatticePolicy` 通过 `PeerSelector`（label 选择器）和 `IPBlock`（CIDR）来描述策略主体。策略在执行时被 `PolicyEvaluator` 解析为 Peer 名称 → overlay IP → iptables 规则。

这意味着策略的实际执行锚定在 **IP** 上。IP 是网络拓扑的偶发属性，不是设备的稳定身份，带来以下问题：

- 设备 IP 重分配后，旧策略可能误放行或误拦截
- 设备替换（新机器 enroll）需要人工重写所有引用该设备的策略
- 策略文本难以直接表达业务意图，需反查 IP 表才能理解规则含义

### Zero Trust 核心原则

Zero Trust 的信任决策应基于**身份**（是谁），而不是**位置**（在哪里）。WireGuard 公钥已在传输层提供加密认证，但控制平面的策略层缺少对应的身份抽象。

### 引入 PeerIdentity 的三大好处

**1. 策略与网络拓扑解耦**

策略引用逻辑角色名（如 `prod-db`），而非 IP 地址。设备迁移、IP 重分配均不影响策略有效性。Lattice 控制平面负责在执行时将角色名解析为当前正确的 IP。

**2. 设备生命周期与策略生命周期独立**

| 事件 | IP-based 结果 | Identity-based 结果 |
|------|-------------|-------------------|
| 设备迁移到新 IP | 引用该 IP 的策略失效 | 策略不变，controller 自动重新解析 |
| 设备替换（换新机器） | 策略需人工重写 | 只改 `PeerIdentity.spec.peerRef`，策略不动 |
| WG 密钥周期轮换 | 无影响 | 无影响（publicKey 变，PeerIdentity 不动） |

**3. 可审计、可理解的意图表达**

```yaml
# 一眼可读：只有 payment-service 能访问 prod-db 的 5432
ingress:
  - from:
    - identityRef: payment-service
  ports:
    - port: 5432
```

对安全审计和合规检查（SOC2、ISO27001）有直接价值，审计员无需翻查 IP 表即可理解策略意图。

---

## 身份模型

### 三层分离

```
PeerIdentity（稳定逻辑身份）
    └── peerRef ──→ LatticePeer（设备注册，设备生命周期）
                        └── spec.publicKey（WG 会话密钥，周期轮换）
                        └── status.allocatedAddress（overlay IP）
```

- **PeerIdentity**：逻辑角色，永久存在，如 `prod-db`、`api-gateway`
- **LatticePeer**：设备注册记录，设备报废时删除
- **WireGuard PublicKey**：会话凭证，周期轮换，不影响上两层

### 全局唯一性

`PeerIdentity` 是 cluster-scoped CRD，但通过 `spec.network` 限定逻辑作用域。唯一键为 `(network, name)` 复合键，不同网络可各自拥有同名身份（如两个网络都可以有 `prod-db`），互不干扰。

`LatticePolicy` 中的 `identityRef` 按 `policy.spec.network` 在相同网络下解析，不会跨网络混淆。

---

## PeerIdentity CRD

### Spec

```go
type PeerIdentitySpec struct {
    // Network 是此 PeerIdentity 归属的 LatticeNetwork 名称。
    // (Network, metadata.name) 构成复合唯一键。
    Network string `json:"network"`

    // PeerRef 是当前绑定的 LatticePeer 名称。
    PeerRef string `json:"peerRef"`

    // PreviousPeerRef 在设备替换时设置，宽限期内旧设备与新设备同时有效。
    // +optional
    PreviousPeerRef string `json:"previousPeerRef,omitempty"`

    // GracePeriodSeconds 是宽限期时长，默认 300s。
    // 宽限期到期后 controller 自动清空 PreviousPeerRef。
    // +kubebuilder:default=300
    GracePeriodSeconds int32 `json:"gracePeriodSeconds,omitempty"`

    // Description 是人类可读的身份描述。
    // +optional
    Description string `json:"description,omitempty"`
}
```

### Status

```go
type PeerIdentityStatus struct {
    // ResolvedPeerIP 是当前 PeerRef 对应的 overlay IP（controller 维护，只读）。
    ResolvedPeerIP string `json:"resolvedPeerIP,omitempty"`

    // PreviousPeerIP 是宽限期内旧设备的 overlay IP。
    PreviousPeerIP string `json:"previousPeerIP,omitempty"`

    // GracePeriodExpiresAt 是宽限期截止时间。
    GracePeriodExpiresAt *metav1.Time `json:"gracePeriodExpiresAt,omitempty"`

    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

### YAML 示例

```yaml
apiVersion: lattice.io/v1alpha1
kind: PeerIdentity
metadata:
  name: prod-db
spec:
  network: prod-network
  peerRef: my-server-v2
  previousPeerRef: my-server-v1   # 设备替换宽限期，可选
  gracePeriodSeconds: 300
  description: "Production database server"
status:
  resolvedPeerIP: "10.0.0.5"
  previousPeerIP: "10.0.0.4"
  gracePeriodExpiresAt: "2026-05-29T12:05:00Z"
```

---

## LatticePolicy 扩展

在 `PeerSelection` 中新增 `identityRef` 字段：

```go
type PeerSelection struct {
    // 现有字段
    PeerSelector *metav1.LabelSelector `json:"peerSelector,omitempty"`
    IPBlock      *IPBlock              `json:"ipBlock,omitempty"`

    // 新增：引用 PeerIdentity 名称（在 policy.spec.network 下解析）
    // +optional
    IdentityRef *string `json:"identityRef,omitempty"`
}
```

三种选择方式可以混用，单条规则内取并集。

### 策略写法示例

```yaml
apiVersion: lattice.io/v1alpha1
kind: LatticePolicy
metadata:
  name: payment-to-db
spec:
  network: prod-network
  peerSelector:
    matchLabels:
      app: payment-service
  ingress:
  - from:
    - identityRef: api-gateway    # 只允许 api-gateway 身份入站
    ports:
    - port: 8080
      protocol: TCP
  egress:
  - to:
    - identityRef: prod-db        # 只允许访问 prod-db 身份
    ports:
    - port: 5432
      protocol: TCP
```

---

## 密钥轮换与设备替换流程

### 场景 A：周期性 WG 密钥轮换（自动，无感知）

```
Agent 生成新密钥对
    → 调用 API 更新 LatticePeer.spec.publicKey
    → peer controller 广播新 AllowedPeer 给网络内所有节点
    → WireGuard 重新握手完成

PeerIdentity 全程不变 → LatticePolicy 全程有效
```

策略影响：**零**。

### 场景 B：设备替换（零停机）

```
1. 新设备 enroll → 创建 LatticePeer "my-server-v2"（新 overlay IP、新 WG 密钥）

2. 管理员执行一次更新：
   PeerIdentity.spec.previousPeerRef = "my-server-v1"   ← 旧设备进入宽限期
   PeerIdentity.spec.peerRef         = "my-server-v2"   ← 新设备立即生效

3. controller 计算：
   resolvedPeerIP = 10.0.0.5（新设备）
   previousPeerIP = 10.0.0.4（旧设备，宽限期内）
   iptables 规则同时包含两个 IP

4. gracePeriodSeconds 到期
   → GracePeriod controller 自动清空 previousPeerRef
   → 旧设备 IP 从 iptables 规则中移除
```

### 场景 C：立即撤销

管理员删除 `PeerIdentity` 或将 `spec.peerRef` 置空，controller 在下次 reconcile 时立即从所有相关策略的 iptables 规则中移除该身份对应的 IP。

---

## Controller 设计

### PeerIdentity Controller

职责：
1. 监听 `PeerIdentity` 变更
2. 解析 `peerRef` → `LatticePeer` → `status.allocatedAddress`，回写 `status.resolvedPeerIP`
3. 若存在 `previousPeerRef`，同样解析并回写 `status.previousPeerIP` 和 `gracePeriodExpiresAt`
4. 回写 `LatticePeer.status.identityRef`（反向索引，便于 agent 自感知）
5. 触发受影响的 `LatticePolicy` 重新 reconcile

### GracePeriod Controller

复用现有 `policy_ttl_controller` 模式，定期扫描 `gracePeriodExpiresAt` 已过期的 `PeerIdentity`，自动清空 `previousPeerRef` 并触发 policy reconcile。

### PolicyEvaluator 扩展

在现有 `resolveRulePeers()` 中新增 identity 解析分支：

```
对每条 PeerSelection:
    若 identityRef != nil:
        查 PeerIdentity WHERE network=policy.network AND name=identityRef
        收集 resolvedPeerIP（必有）
        若 previousPeerIP 存在且宽限期未过期，一并收集
        将 IP 列表注入现有流程（后续 iptables 生成逻辑不变）
```

---

## Agent 自感知

Agent 当前知道自身的 `AppID` 和 WireGuard 公钥，但无法直接得知自己被赋予了哪个逻辑身份。

### 方案：服务端在注册 / 心跳响应中携带 identityName

```
Agent 注册 / 心跳
    ↓
Server: SELECT name FROM t_peer_identity
        WHERE network_id = ? AND peer_ref = thisAppID
    ↓
注册响应附带:
    identityName: "prod-db"   （无则为空）
    ↓
Agent 本地状态记录 identityName
用于：日志结构化字段、审计事件、可观测性指标
```

同时 controller 在 `LatticePeer.status.identityRef` 回写身份名称，供 k8s 侧查询和告警规则使用。

---

## 数据库层（非 k8s 部署）

Lattice 非 k8s 模式下，PeerIdentity 作为 DB 实体存储：

```sql
CREATE TABLE t_peer_identity (
    id                      VARCHAR(36)  PRIMARY KEY,
    network_id              VARCHAR(36)  NOT NULL,
    name                    VARCHAR(200) NOT NULL,
    peer_ref                VARCHAR(200) NOT NULL,
    previous_peer_ref       VARCHAR(200),
    grace_period_seconds    INT          NOT NULL DEFAULT 300,
    resolved_peer_ip        VARCHAR(64),
    previous_peer_ip        VARCHAR(64),
    grace_period_expires_at DATETIME,
    description             VARCHAR(500),
    created_at              DATETIME,
    updated_at              DATETIME,
    UNIQUE KEY idx_network_name (network_id, name)
);
```

---

## 兼容性

- `identityRef` 是 `PeerSelection` 的新增可选字段，现有策略（仅用 `peerSelector` / `ipBlock`）**无需修改，行为不变**
- `PeerIdentity` 不存在时，`identityRef` 解析结果为空列表，等同于该规则匹配零个 peer（安全失败，不放行）
- 渐进式迁移：可逐步将高价值策略从 `peerSelector` 迁移到 `identityRef`，两者可共存于同一条规则

---

## 实现范围

本设计覆盖：

1. `PeerIdentity` CRD 定义（`api/v1alpha1/`）
2. `PeerIdentity` controller（`internal/agent/controller/`）
3. GracePeriod controller（复用 TTL controller 模式）
4. `PeerSelection.identityRef` 字段扩展（`api/v1alpha1/`）
5. `PolicyEvaluator.resolveRulePeers()` identity 解析分支
6. Server 注册/心跳响应携带 `identityName`
7. DB 表 `t_peer_identity` 及对应 GORM model / service 层
8. `LatticePeer.status.identityRef` 回写

不在本设计范围内：

- SPIFFE/SPIRE 证书集成（未来扩展）
- OPA 动态策略引擎集成（未来扩展）
- 设备健康门控（Posture Check，未来扩展）
