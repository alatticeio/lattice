# Lattice AI Agent Zero Trust 安全网络设计

**日期**：2026-05-30  
**状态**：Draft  
**关联文档**：[AI Agent Secure Mesh](./2026-05-29-ai-agent-secure-mesh-design.md)、[Agent Sandbox](./2026-05-29-agent-sandbox-design.md)

---

## 产品定位

**Lattice Mesh**：AI Agent 时代的 Zero Trust 安全网络基础设施

让 AI Agent 安全地：
- 访问任何资源（数据库、API、内部系统）
- 调用任何工具（文件、命令、K8s）
- 与任何 Agent 协作
- 运行在任何位置（云、边缘、设备）

**一句话定位**：Tailscale for AI Agents

---

## 背景与动机

### 现状问题

#### 网络层问题

Lattice 现有 `LatticePolicy` 通过 `PeerSelector`（label 选择器）和 `IPBlock`（CIDR）来描述策略主体。策略在执行时被 `PolicyEvaluator` 解析为 Peer 名称 → overlay IP → iptables 规则。

这意味着策略的实际执行锚定在 **IP** 上。IP 是网络拓扑的偶发属性，不是设备的稳定身份，带来以下问题：

- 设备 IP 重分配后，旧策略可能误放行或误拦截
- 设备替换（新机器 enroll）需要人工重写所有引用该设备的策略
- 策略文本难以直接表达业务意图，需反查 IP 表才能理解规则含义

#### AI Agent 层问题

传统 VPN/overlay 网络（Tailscale、ZeroTier）解决的是**人与机器**的连接。AI Agent 带来新的安全挑战：

- **身份问题**：AI Agent 是动态的、短生命周期的，传统用户账号不适用
- **权限问题**：Agent 需要细粒度权限（不能给整个 VPN），需要工具级 RBAC
- **审计问题**：企业需要知道 AI Agent 做了什么，满足 SOC2/ISO27001 合规要求
- **隔离问题**：Agent 可能执行恶意代码或访问敏感资源，需要沙箱隔离

### Zero Trust 核心原则

Zero Trust 的信任决策应基于**身份**（是谁），而不是**位置**（在哪里）。WireGuard 公钥已在传输层提供加密认证，但控制平面的策略层缺少对应的身份抽象。

### 引入 Zero Trust 的三大好处

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

## 核心架构

### 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    Lattice Cloud 控制面                          │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │  身份服务    │  │  策略引擎    │  │  审计服务    │            │
│  │ AgentIdentity│  │ PeerIdentity│  │  审计日志    │            │
│  │  API Key    │  │ LatticePolicy│  │  合规报告    │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│         │                │                │                     │
│  ┌──────┴────────────────┴────────────────┴──────┐             │
│  │                   NATS 信令层                  │             │
│  │        (身份广播、策略同步、Agent 发现)         │             │
│  └───────────────────────────────────────────────┘             │
└─────────────────────────────────────────────────────────────────┘
              │                              │
              ▼                              ▼
┌─────────────────────────┐    ┌─────────────────────────┐
│   用户环境 A（云）       │    │   用户环境 B（边缘）     │
│                         │    │                         │
│  ┌─────┐    ┌─────┐    │    │  ┌─────┐    ┌─────┐    │
│  │Agent│    │Agent│    │    │  │Agent│    │Agent│    │
│  │ (AI)│    │(工具)│    │    │  │(IoT)│    │(边缘)│    │
│  └──┬──┘    └──┬──┘    │    │  └──┬──┘    └──┬──┘    │
│     │          │       │    │     │          │       │
│     └────┬─────┘       │    │     └────┬─────┘       │
│          │             │    │          │             │
│    ┌─────┴─────┐       │    │    ┌─────┴─────┐       │
│    │  沙箱环境  │       │    │    │  沙箱环境  │       │
│    │ (gVisor)  │       │    │    │ (轻量)    │       │
│    └───────────┘       │    │    └───────────┘       │
└─────────────────────────┘    └─────────────────────────┘
```

### 身份模型（四层分离）

```
AgentIdentity（AI Agent 身份）
    └── peerRef ──→ LatticePeer（设备注册）
                        └── PeerIdentity（设备角色）
                            └── spec.publicKey（WG 会话密钥）
```

- **AgentIdentity**：AI Agent 的身份，绑定工具权限和沙箱模式
- **PeerIdentity**：设备的逻辑角色，如 `prod-db`、`factory-gateway`
- **LatticePeer**：设备注册记录，设备报废时删除
- **WireGuard PublicKey**：会话凭证，周期轮换，不影响上三层

---

## 身份层设计

### AgentIdentity（AI Agent 身份）

#### 问题

AI Agent 是动态的、短生命周期的，传统身份（用户账号）不适用。需要为每个 Agent 颁发独立身份。

#### AgentIdentity CRD

```go
type AgentIdentitySpec struct {
    // PeerRef 是此 Agent 运行所在的 LatticePeer 名称。
    PeerRef string `json:"peerRef"`

    // AllowedTools 是工具名称白名单。
    // 空列表表示不允许任何工具。
    AllowedTools []string `json:"allowedTools,omitempty"`

    // AllowedNamespaces 限制 Agent 工具调用可影响的 K8s 命名空间。
    AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`

    // Sandbox 控制此 Agent 的托管隔离模式。
    // +kubebuilder:default=none
    Sandbox SandboxMode `json:"sandbox,omitempty"`

    // AuditLevel 控制工具调用日志详细程度。
    // +kubebuilder:default=write
    AuditLevel AuditLevel `json:"auditLevel,omitempty"`

    // EnforcementMode 可覆盖全局 agent-isolation 强制模式。
    // +kubebuilder:default=enforce
    EnforcementMode EnforcementMode `json:"enforcementMode,omitempty"`

    // ExpiresAt 是此 AgentIdentity 的可选过期时间。
    // 过期后 controller 将阶段转换为 Expired。
    ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

    // ParentRef 是父 AgentIdentity 名称（子 Agent 场景）。
    ParentRef string `json:"parentRef,omitempty"`

    // SpawnableRoles 列出此 Agent 可作为子 Agent 生成的角色模板名称。
    SpawnableRoles []string `json:"spawnableRoles,omitempty"`
}

type AgentIdentityStatus struct {
    // Phase 是当前生命周期阶段。
    Phase AgentPhase `json:"phase,omitempty"`

    // PeerIP 是此 Agent 的 LatticePeer 分配的 VPN IP。
    PeerIP string `json:"peerIP,omitempty"`

    // LastSeenAt 是 Agent 最后一次认证 API 调用的时间。
    LastSeenAt *metav1.Time `json:"lastSeenAt,omitempty"`

    // Conditions 反映控制面同步状态。
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

#### YAML 示例

```yaml
apiVersion: lattice.io/v1alpha1
kind: AgentIdentity
metadata:
  name: payment-agent
spec:
  peerRef: agent-pod-abc123
  allowedTools:
    - "db:query"
    - "api:call"
    - "file:read"
  allowedNamespaces:
    - "production"
    - "staging"
  sandbox: gvisor
  auditLevel: full
  enforcementMode: enforce
  expiresAt: "2026-06-01T00:00:00Z"
status:
  phase: Active
  peerIP: "10.0.0.5"
  lastSeenAt: "2026-05-30T10:15:30Z"
```

#### 身份生命周期

```
Agent 启动 → 申请身份 → 控制面颁发 → 运行时验证 → 过期/撤销
```

---

### PeerIdentity（设备身份）

#### 问题

Agent 运行在物理设备上，设备身份与 Agent 身份需要分离。同一设备可运行多个 Agent，各有独立身份和权限。

#### PeerIdentity CRD

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

#### YAML 示例

```yaml
apiVersion: lattice.io/v1alpha1
kind: PeerIdentity
metadata:
  name: prod-db
spec:
  network: prod-network
  peerRef: my-server-v2
  previousPeerRef: my-server-v1
  gracePeriodSeconds: 300
  description: "Production database server"
status:
  resolvedPeerIP: "10.0.0.5"
  previousPeerIP: "10.0.0.4"
  gracePeriodExpiresAt: "2026-05-30T12:05:00Z"
```

---

## 策略引擎设计

### LatticePolicy 扩展

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
    - identityRef: api-gateway
    ports:
    - port: 8080
      protocol: TCP
  egress:
  - to:
    - identityRef: prod-db
    ports:
    - port: 5432
      protocol: TCP
```

### OPA 动态策略（未来扩展）

```rego
# 动态策略（OPA）
allow if {
    agent.identity == input.agent_id
    agent.sandbox == "gvisor"
    agent.posture.disk_encrypted == true
    time.now_ns() < agent.session_expires_at
    agent.risk_score < 80
}
```

---

## 沙箱隔离设计

### 问题

AI Agent 可能执行恶意代码或访问敏感资源，需要多层沙箱隔离。

### 沙箱模式

| 模式 | 描述 | 适用场景 |
|------|------|---------|
| `none` | 无沙箱 | 开发环境 |
| `pod` | K8s Pod + seccomp | 社区版 |
| `gvisor` | gVisor 用户态内核 | Pro 版 |
| `microvm` | Firecracker MicroVM | 未来 |

### 架构

```
┌─────────────────────────────────────────┐
│              宿主机                      │
│  ┌─────────────────────────────────┐   │
│  │         gVisor (用户态内核)      │   │
│  │  ┌─────────────────────────┐   │   │
│  │  │      Agent 进程          │   │   │
│  │  │  ┌─────┐  ┌─────┐      │   │   │
│  │  │  │工具A │  │工具B │      │   │   │
│  │  │  └─────┘  └─────┘      │   │   │
│  │  └─────────────────────────┘   │   │
│  └─────────────────────────────────┘   │
│                                         │
│  ┌─────────────────────────────────┐   │
│  │       eBPF TC Hook (网络策略)    │   │
│  │   - 按 AgentIdentity 过滤       │   │
│  │   - 按工具类型过滤              │   │
│  │   - 实时审计日志                │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

---

## 审计与合规设计

### 问题

企业需要知道 AI Agent 做了什么，满足 SOC2/ISO27001 合规要求。

### 审计日志格式

```json
{
  "timestamp": "2026-05-30T10:15:30Z",
  "agent_id": "payment-agent",
  "peer_id": "device-abc",
  "peer_identity": "production-db-server",
  "action": "tool_call",
  "tool": "db:query",
  "target": "SELECT * FROM orders WHERE user_id = 123",
  "result": "success",
  "rows_affected": 5,
  "sandbox": "gvisor",
  "policy_matched": "payment-to-db",
  "risk_score": 15
}
```

### 审计能力

- **实时审计流**：WebSocket/SSE 推送
- **历史查询**：按 Agent、工具、时间范围
- **合规报告导出**：SOC2、ISO27001 模板
- **异常告警**：风险评分超阈值

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

## 核心场景实现

### 场景 1：Agent 访问内部资源

```
1. Agent 启动，申请 AgentIdentity
2. 控制面颁发身份 + 权限白名单
3. Agent 通过 Lattice 网络访问数据库
4. 策略引擎检查：
   - AgentIdentity 是否有效？
   - 是否有 db:query 权限？
   - 目标是否在 allowedNamespaces？
5. 通过 → 放行，记录审计日志
6. 不通过 → 拒绝，记录告警
```

### 场景 2：Agent 工具调用安全

```
1. Agent 调用工具（如执行命令）
2. 工具代理（MCP Proxy）拦截请求
3. 策略引擎检查：
   - 工具是否在 allowedTools 白名单？
   - 参数是否符合策略？（如不能 rm -rf /）
   - 沙箱是否足够隔离？
4. 通过 → 在沙箱内执行
5. 不通过 → 拒绝，记录审计日志
```

### 场景 3：Agent 间通信

```
1. Agent A 需要调用 Agent B
2. 通过 Lattice 网络发起请求
3. 策略引擎检查：
   - Agent A 的 AgentIdentity 是否有效？
   - Agent A 是否有权调用 Agent B？
   - 通信是否需要加密？
4. 通过 → 建立 WireGuard 隧道
5. 不通过 → 拒绝，记录审计日志
```

### 场景 4：Agent 边缘部署

```
1. 边缘设备启动，注册 LatticePeer
2. PeerIdentity 绑定设备角色（如 "factory-gateway"）
3. Agent 部署到边缘设备
4. AgentIdentity 绑定到设备 + Agent
5. 策略引擎检查：
   - 设备健康状态？（OS 版本、补丁、EDR）
   - Agent 权限是否匹配设备角色？
6. 通过 → Agent 正常运行
7. 不通过 → Agent 降级或拒绝启动
```

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

### PeerIdentity 表

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

### AgentIdentity 表

```sql
CREATE TABLE t_agent_identity (
    id                  VARCHAR(36)  PRIMARY KEY,
    tenant_id           VARCHAR(36)  NOT NULL,
    peer_ref            VARCHAR(200) NOT NULL,
    allowed_tools       TEXT,        -- JSON: ["db:query","api:call"]
    allowed_namespaces  TEXT,        -- JSON: ["production","staging"]
    sandbox             VARCHAR(50)  NOT NULL DEFAULT 'none',
    audit_level         VARCHAR(50)  NOT NULL DEFAULT 'write',
    enforcement_mode    VARCHAR(50)  NOT NULL DEFAULT 'enforce',
    expires_at          DATETIME,
    parent_ref          VARCHAR(200),
    spawnable_roles     TEXT,        -- JSON: ["role1","role2"]
    phase               VARCHAR(50)  NOT NULL DEFAULT 'Pending',
    peer_ip             VARCHAR(64),
    last_seen_at        DATETIME,
    created_at          DATETIME,
    updated_at          DATETIME,
    INDEX idx_tenant_peer (tenant_id, peer_ref)
);
```

---

## 兼容性

- `identityRef` 是 `PeerSelection` 的新增可选字段，现有策略（仅用 `peerSelector` / `ipBlock`）**无需修改，行为不变**
- `PeerIdentity` 不存在时，`identityRef` 解析结果为空列表，等同于该规则匹配零个 peer（安全失败，不放行）
- 渐进式迁移：可逐步将高价值策略从 `peerSelector` 迁移到 `identityRef`，两者可共存于同一条规则

---

## 竞争优势

| 能力 | Lattice | Tailscale | Istio | 传统 VPN |
|------|---------|-----------|-------|----------|
| Agent 身份管理 | ✅ | ❌ | ❌ | ❌ |
| 工具级 RBAC | ✅ | ❌ | ❌ | ❌ |
| 沙箱隔离 | ✅ | ❌ | ❌ | ❌ |
| 审计日志 | ✅ | 基础 | 基础 | ❌ |
| 任意设备支持 | ✅ | ✅ | ❌ | ✅ |
| P2P 直连 | ✅ | ✅ | ❌ | ❌ |
| K8s 无关 | ✅ | ✅ | ❌ | ✅ |

---

## 商业模式

### 免费版（个人开发者）

- 5 个 Agent
- 基础身份管理
- 社区支持

### Pro 版（$49/月）（创业公司）

- 50 个 Agent
- 完整 RBAC + 审计
- 沙箱隔离（gVisor）
- 邮件支持

### Enterprise 版（$199/月起）（大企业）

- 无限 Agent
- BYOC 部署
- OPA 动态策略
- 合规报告
- SSO/SAML
- 专属支持

---

## 实现范围

### Phase 1（2 个月）：MVP

1. `AgentIdentity` CRD 稳定
2. `PeerIdentity` CRD 实现
3. 基础 RBAC（工具白名单）
4. 审计日志（基础版）
5. 云控制面 MVP

### Phase 2（4 个月）：企业级

1. 沙箱隔离（gVisor）
2. OPA 策略引擎集成
3. 合规报告
4. 多区域部署

### Phase 3（6 个月）：规模化

1. 边缘设备支持
2. Agent 间通信
3. 高级审计（异常检测）
4. BYOC 部署

---

## 实现计划

详细实现步骤见：[2026-05-29-peer-identity-zero-trust-plan.md](../plans/2026-05-29-peer-identity-zero-trust-plan.md)
