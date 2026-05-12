# AI Agent 安全隔离设计

> 本文档记录 Lattice 项目中 AI Agent 安全隔离的背景分析、场景定义、现状评估和设计方案。
> 日期：2026-05-09

---

## 零、为什么 Lattice 该做这件事

### 0.1 未来不是"一个全能 AI"，而是"Agent 蜂群"

随着 AI Agent 爆发，未来的主流形态不会是一个全知全能的超级 AI，而是**每个产品都有自己的 Agent**，它们像蜂群一样协作：

- CRM 有自己的销售 Agent
- 代码仓库有自己的 Code Review Agent
- 财务软件有自己的对账 Agent
- IoT 平台有自己的算力调度 Agent

这些 Agent 不仅在本地运行，还会**频繁通过网络访问外部 API、互相协作**。典型场景：

```
用户："帮我把最新的 115 网盘里的 PDF 整理成周报发到飞书"

Agent 自动：
  → 获取 115 网盘访问权限，拉取 PDF
  → 调用 PDF 处理服务提取内容
  → 调用飞书 API 生成并发送周报
  → 三个外部服务，三套认证，全程无人值守
```

### 0.2 现有架构层的尴尬

| 层 | 问题 |
|----|------|
| 应用层 | 只管业务逻辑，不管底层安全；每个 Agent 框架各自实现，标准不统一 |
| 操作系统层 | 权限粒度太粗（按进程或用户），无法理解"这次访问的意图是什么" |
| **Lattice 层（Network/Kernel）** | **唯一能做到"身份 + 意图 + 流量"三位一体管控的层** |

只有在 Lattice 这一层，才能：
- 识别是**哪个 Agent 实例**在发出流量（不是哪个进程或用户）
- 理解该流量背后的**意图**（通过 Intent Engine）
- 在**数据包级别**实施精细控制（通过 eBPF TC hook）

---

## 一、什么是 AI Agent

**普通 LLM**：用户问一句，模型答一句，结束。

**AI Agent**：用户给一个目标，Agent 自主决定调用哪些工具、调用多少次、按什么顺序，直到完成目标。

```
用户："帮我排查为什么 frontend 连不上 api-gateway"

Agent 自主执行：
  1. list_peers()           → 看看有哪些节点
  2. check_connectivity()   → 确认确实不通
  3. list_policies()        → 检查策略
  4. list_snapshots()       → 查历史变更
  5. diff_snapshots()       → 发现昨天删了一条策略
  6. 输出根因报告
```

Lattice 现有的 `ai.go` 就是这种模式——有工具调用能力的 Agent，工具集固定，写操作走人工审批。

---

## 二、AI Agent 的三个发展阶段

### 阶段一：工具调用 Agent（Lattice 当前状态）

Agent 调用**预定义的工具**，工具由平台实现和控制。

```
Agent ──调用──> list_peers / create_policy / check_connectivity
                       |
               平台执行，结果返回 Agent
```

**特征**：工具集固定、平台托管执行、无任意代码运行

**主要风险**：
- Prompt Injection：恶意用户构造输入绕过审批流
- 权限越界：Agent 操作了它不该操作的工作区资源

**需要的隔离**：
- 工具调用白名单（哪些 Agent 能调用哪些工具）
- 写操作走审批流（已有 WorkflowService）
- 用户输入校验

---

### 阶段二：自治 Agent（未来 1-2 年主流）

Agent 不仅调用工具，还能**持久运行、定时触发、自主决策**，不再等人问。

**典型场景**：
- 每天凌晨扫描网络，发现未授权 Peer 自动隔离
- 监控所有节点流量，发现异常自动告警
- Agent A 发现问题，通知 Agent B 协同处理

**新增风险**：
- Agent 被 Prompt Injection 后会**持续执行**恶意操作（不是一次性）
- 多 Agent 互相通信时，一个被攻击可能传播
- 操作不受人直接监督，出问题难追溯

**需要的隔离**：
- 每个 Agent 有独立的加密身份，不能冒充其他 Agent
- Agent 间通信需要明确授权
- 完整的操作审计日志
- 异常行为检测与熔断机制

---

### 阶段三：代码执行 Agent（MicroVM/eBPF 真正必要的场景）

Agent 能运行**任意代码**：执行 Shell 命令、运行用户上传的脚本、调用外部服务。

**典型场景**：
- 在 Peer 节点上运行诊断脚本
- 自动部署代码到节点
- 用户上传自定义 Agent 到平台运行

**风险等级**：极高。Agent 一旦被攻击或行为异常：
- 可能读取宿主机敏感文件（SSH 密钥、TLS 证书）
- 可能横向移动到其他节点
- 可能破坏整个基础设施

**这个阶段才真正需要**：
- MicroVM（Firecracker）：每个 Agent 独立内核，逃逸极难
- eBPF 系统调用监控：内核层面不可见监控，Agent 无法绕过
- seccomp profile：限制可用系统调用集合

---

## 三、Lattice 当前 AI 架构现状

### 已有组件

| 组件 | 路径 | 作用 |
|------|------|------|
| LLM 客户端 | `internal/server/llm/` | Anthropic / OpenAI-compat 多 Provider |
| AI 服务 | `internal/server/service/ai.go` | Chat / Audit / Debug + 14 个工具 |
| 审批流 | `internal/server/service/workflow.go` | 写操作 Submit/Approve/Reject |
| 意图引擎 | `internal/server/service/intent_pro.go` | 自然语言 → CRD 变更计划（Pro）|
| 策略执行 | `internal/agent/provision/` | iptables（社区）/ eBPF TC（Pro）|

### 已有的隔离能力

- **网络层**：WireGuard + LatticePolicy 实现零信任网络隔离（每个 Peer 有独立密钥对）
- **写操作审批**：`submitOrApply()` 路由写操作到 WorkflowService
- **工具预算**：`maxToolCalls` 限制单次 agentic loop 的工具调用次数
- **命名空间 scoping**：工具调用限定在 workspace namespace 内

### 当前缺口

| 缺口 | 影响 |
|------|------|
| Agent 没有独立身份 | 所有 AI 操作以 `"ai-agent"` 固定身份提交，无法区分是哪个 Agent 做的 |
| 工具调用无 RBAC | 所有能访问 Chat 接口的用户都有完整工具集 |
| 工具调用未写审计日志 | 出问题无法追溯 Agent 做了什么 |
| 多租户 namespace 校验在客户端 | namespace 参数由调用方传入，服务端未强制校验归属 |
| 无速率限制 | AI 接口无防滥用保护 |

---

## 四、场景定义（C 型：混合部署）

Lattice 规划的 Agent 托管平台为 **C 型**：

- Agent 可以跑在**任意地方**（用户自己的机器、开发者笔记本、移动端、边缘节点或 Lattice 托管节点）
- 通过 Lattice WireGuard Overlay 作为 Peer 接入，获得网络身份
- 平台同时提供**沙箱托管选项**（用户可选，社区版 Pod，Pro 版 MicroVM）
- 隔离重点在于：**网络边界 + 身份绑定 + 控制面权限**

### 典型协作场景

**场景一：企业内部流程 Agent 蜂群**
```
Bug Monitor Agent（监控 Jira）
  ↓ 发现 P0 Bug，触发
Fix Agent（在构建环境修复代码）
  ↓ 修复完成，触发
Test Agent（在测试网运行回归）
  ↓ 通过，触发
Deploy Agent（推送到预发）

三个 Agent 需要在开发网 / 测试网 / 预发网之间穿梭
每次穿梭都需要 Lattice 验证身份 + 动态开通对应路径
```

**场景二：跨产品协作**
```
用户："把 115 网盘最新 PDF 整理成周报发到飞书"

单个 Agent 需同时获得：
  - 115 网盘 API 访问权（读取）
  - PDF 处理服务访问权（计算）
  - 飞书 API 访问权（写入）

Lattice：为每个权限段创建 Ephemeral Policy，任务完成全部销毁
```

**场景三：边缘计算 Agent**
```
IoT / GPU 节点上的 Agent 自主调度算力资源
Agent 物理上分散在全球节点，但通过 Lattice Overlay 统一管控
无视物理网络拓扑，建立统一的安全边界
```

### 阶段映射

| 时间线 | Agent 阶段 | 主要需求 |
|--------|-----------|---------|
| 当前 | 阶段一（工具调用） | 工具 RBAC + 审批流（部分已有）|
| 短期 | 阶段一 + 二 | Agent 身份 + 网络隔离 + 审计 |
| 长期 | 阶段二 + 三 | 沙箱托管 + eBPF 监控（Pro）|

---

## 五、三个设计方案

### 方案 A：Overlay-Native（轻量）

**思路**：Agent = LatticePeer，网络隔离完全复用 WireGuard + LatticePolicy，控制面加 Agent JWT Scoping。

```
Agent 启动
  → 向 LatticeD 注册，获得 WireGuard 密钥对 + Agent JWT
  → 作为 LatticePeer 接入 Overlay，分配 VPN IP
  → 网络访问受 LatticePolicy 控制（默认拒绝）
  → 调用 AI 工具时，JWT 携带 agent_id + allowed_namespaces
  → 托管 Agent：K8s Pod + seccomp（社区版）
```

**优点**：复用全部现有基础设施，改动量最小，3-4 周可出原型
**缺点**：无内核级监控；容器隔离共享宿主内核

---

### 方案 B：Overlay + Agent 身份层（推荐）

**思路**：引入 `AgentIdentity` CRD，将网络身份（WireGuard）和 API 身份（RBAC）统一管理。

```yaml
# AgentIdentity CRD 完整字段定义
apiVersion: lattice.io/v1alpha1
kind: AgentIdentity
metadata:
  name: agent-monitoring-prod         # 唯一标识
  namespace: workspace-a
spec:
  # 网络身份绑定
  peerRef: peer-monitoring-prod       # 对应的 LatticePeer 名称（必填）

  # 控制面权限
  allowedTools:                       # 工具白名单（空列表 = 拒绝所有工具）
    - list_peers
    - list_policies
    - check_connectivity
    # Write 类工具不在此列，该 Agent 为只读
  allowedNamespaces:                  # 可操作的 namespace（空列表 = 拒绝所有）
    - workspace-a

  # 沙箱配置（托管模式）
  sandbox: pod                        # pod | microvm | none（非托管）

  # 可观测性
  auditLevel: full                    # none | write | full（full = 记录所有调用）
  enforcementMode: enforce            # disabled | audit | enforce（可覆盖全局配置）

  # 生命周期
  expiresAt: "2027-01-01T00:00:00Z"  # Agent 身份过期时间（可选，空 = 永不过期）

status:
  # 由控制器填写，只读
  phase: Active                       # Pending | Active | Expired | Revoked
  peerIP: "10.100.1.5"               # 分配的 VPN IP
  lastSeenAt: "2026-05-09T14:30:00Z" # 最后活跃时间
  conditions:
    - type: PeerBound                 # LatticePeer 是否已绑定
      status: "True"
    - type: JWTIssued                 # JWT 是否已签发
      status: "True"
```

```
网络层：    LatticePolicy（已有，不改）
身份层：    AgentIdentity CRD + Agent JWT（新增）
控制面：    ExecuteTool() 校验 agent_id scope（改造）
托管沙箱：  社区版 K8s Pod + seccomp
            Pro 版 Firecracker MicroVM（独立内核）
审计：      所有工具调用写入 AuditLog（扩展现有 audit.go）
```

**优点**：身份和权限统一管理；与现有 CRD 体系一致；社区/Pro 两档沙箱
**缺点**：需要新 CRD + admission webhook；MicroVM 集成复杂度较高

---

### 方案 C：全栈隔离（重量级）

在方案 B 基础上增加：
- eBPF TC hook 监控 Agent 进程的系统调用和网络行为（扩展现有 `PolicyEnforcer` 接口）
- GovernanceAgent 统一监管多个执行 Agent（1+3 Grid Cell 模式）
- SPIFFE/SPIRE 做跨集群 Agent 身份验证

**优点**：防御深度最强，适合高敏感场景
**缺点**：工程量是方案 B 的 3-4 倍；eBPF 仅 Linux 5.10+；当前阶段过重

---

## 六、推荐方案及理由

**推荐方案 B**。

**核心判断**：文章描述的 MicroVM + eBPF 是针对**执行任意代码**的 Agent（类 Devin/Claude Code）。Lattice 的 Agent 即便在托管模式下，操作的仍是 Lattice 的控制面 API，不是任意 shell 命令。容器隔离（K8s Pod + seccomp）对当前 80% 场景已经足够。

**技术优先级**：

| 技术 | 优先级 | 阶段 |
|------|--------|------|
| WireGuard 身份绑定（Agent as Peer）| 必须，现在做 | P0 |
| LatticePolicy 网络隔离 | 必须，已有 | P0 |
| Agent JWT + 工具调用 RBAC | 必须，现在做 | P0 |
| 工具调用审计日志 | 必须，现在做 | P0 |
| namespace 归属服务端强制校验 | 必须，现在做 | P0 |
| 速率限制（AI 接口）| 应该做 | P1 |
| K8s Pod + seccomp 沙箱（托管）| 应该做 | P1 |
| `AgentIdentity` CRD | **必须，现在做** | **P0** |
| eBPF 系统调用监控 | 暂缓，Pro 路线图 | P2 |
| Firecracker MicroVM | 暂缓，Pro 路线图 | P2 |
| 1+3 Grid Cell 治理 | 暂缓 | P3 |

---

## 七、工具分类标准

Agent RBAC 依赖明确的工具分类。以下是 `ai.go` 中 14 个工具的完整分类：

| 分类 | 工具名 | 说明 |
|------|--------|------|
| **读操作**（Read） | `list_peers` | 列出所有 Peer，无副作用 |
| | `list_policies` | 列出所有策略 |
| | `list_networks` | 列出所有网络 |
| | `check_connectivity` | 检查两 Peer 连通性 |
| | `list_snapshots` | 列出历史快照索引（Pro）|
| | `get_snapshot` | 获取快照详情（Pro）|
| | `diff_snapshots` | 对比两快照差异（Pro）|
| | `check_connectivity_at` | 历史时间点连通性（Pro）|
| **写操作**（Write，需审批） | `create_policy` | 创建访问策略 |
| | `delete_policy` | 删除访问策略 |
| | `create_peer` | 创建 Peer 节点 |
| | `delete_peer` | 删除 Peer 节点 |
| **意图操作**（Intent，Pro + 需审批） | `plan_network_change` | 生成变更计划（不立即执行）|
| | `apply_network_change` | 执行已审批的变更计划 |

**AgentIdentity 的 `allowedTools` 字段使用此分类**：
- 只读 Agent（如监控 Agent）：仅允许 Read 类工具
- 运维 Agent：允许 Read + Write，所有 Write 操作仍走审批流
- 意图 Agent（Pro）：允许全部，但 Intent 类需额外 Pro 授权

---

## 八、三层隔离机制

### 8.1 身份感知隔离（Identity-Based Isolation）

**不按 IP 隔离，按 NHI（Lattice 网络身份）隔离。**

当 Claude Code 的 Agent 在你的 Mac 上启动时，Lattice 识别出它的身份（WireGuard 密钥对），在内核层为它划定一个"隐形围栏"。它只能看到该任务所需的网络路径，看不见本地运行的其他服务。

即使攻击者拿到了这台 Mac 的权限，也无法伪造 Agent 身份发出受控之外的请求——因为身份绑定的是**加密密钥**，不是 IP 地址。

实现：`AgentIdentity` CRD 绑定 LatticePeer WireGuard 密钥对（见方案 B）。

### 8.2 意图驱动的动态权限（Intent-Driven Ephemeral Access）

Agent 声明意图 → Lattice 验证 → 动态开启一次性加密通道 → 任务结束通道销毁。

详见下一节。

### 8.3 数据平面加密隔离（Data Plane Encryption per Agent Cluster）

**每个产品的 Agent 簇拥有独立的 WireGuard 虚拟网段。**

```
财务 Agent 簇   → WireGuard 网段 10.100.1.0/24，独立密钥组
研发 Agent 簇   → WireGuard 网段 10.100.2.0/24，独立密钥组
CRM Agent 簇    → WireGuard 网段 10.100.3.0/24，独立密钥组
```

即使财务 Agent 和研发 Agent 跑在同一台服务器上，它们发出的数据包在**不同的加密隧道**中传输，物理上互不可见。跨簇通信必须通过 Lattice 控制面显式授权。

实现：每个产品的 Agent 簇对应一个独立的 `LatticeNetwork`，网段隔离由现有 WireGuard 机制保证，无需额外开发。

---

## 九、意图驱动的动态权限（Ephemeral Access）

### 9.1 概念

这是方案 B 的核心扩展功能，正式名称为 **Just-In-Time (JIT) Access + Ephemeral Policy**。

**传统权限模型**：预先分配静态权限，Agent 拥有后一直持有，出问题难以收回。

**Ephemeral Access 模型**：Agent 声明"我需要访问 X"，Lattice 验证后动态开启一条一次性加密通道，任务结束通道立即销毁，Agent 无法通过该通道进行任何计划外的横向移动。

### 9.2 与现有架构的契合度

拼图几乎都已存在：

```
Agent 声明意图
  → IntentService.Plan()     ← 已有：自然语言 → CRD 变更计划
  → 风险评估（RiskLevel）     ← 已有：IntentPlanView.RiskLevel 字段
  → IntentService.Apply()    ← 已有：提交 WorkflowRequest
  → LatticePolicy 创建       ← 已有：CRD 本身
  → eBPF TC hook 执行        ← 已有（Pro）：tc_ingress.bpf.c
  → TTL 到期自动销毁          ← 缺：需加 expiresAt 字段 + GC Controller
```

**唯一缺失的两个组件**：
1. `LatticePolicy.spec.expiresAt` 字段（TTL）
2. GC Controller：watch 过期 policy 并自动删除

### 9.3 完整流程

```
1. Agent 发起访问请求
   POST /api/v1/agents/{id}/access-request
   {
     "intent": "我需要访问 peer-storage-01 的 WebDAV (port 5005)，用于备份配置，预计 30 分钟",
     "duration": "30m"
   }

2. Lattice 意图验证
   - LLM 解析意图：目标 peer、端口、协议、用途
   - 规则引擎评估风险：
       低风险（单点、已知端口、短时间）→ 自动审批
       高风险（广播、敏感端口、超 2 小时）→ 人工审批

3. 创建 Ephemeral LatticePolicy
   kind: LatticePolicy
   metadata:
     name: ephemeral-agent-x-storage-20260509t1430
     labels:
       lattice.io/ephemeral: "true"
       lattice.io/agent-id: "agent-x"
   spec:
     network: prod-network
     action: ALLOW
     expiresAt: "2026-05-09T15:00:00Z"   ← 新增字段
     peerSelector:
       matchLabels:
         app: storage-01
     ingress:
       - from:
           - peerSelector:
               matchLabels:
                 agent.lattice.io/id: agent-x
         ports:
           - port: 5005
             protocol: TCP

4. eBPF TC hook 立即生效
   通道开启，仅 agent-x 可访问 storage-01:5005，其他所有流量仍被拒绝

5. TTL 到期 或 任务提前完成
   GC Controller 删除 LatticePolicy → eBPF 规则自动撤销 → 通道消失

6. 审计记录全程
   - 谁申请、申请什么、持续多久、实际访问了哪些地址和端口
```

### 9.4 解决的核心威胁

| 威胁 | 应对方式 |
|------|---------|
| Agent 被 Prompt Injection，尝试横向移动 | 通道是单点定向的，移动到其他 Peer 的请求被 eBPF 直接丢弃 |
| 权限蔓延（权限被长期持有） | TTL 强制到期，无需人工清理 |
| 任务结束后 Agent 继续探测 | 任务完成可主动调用 revoke，不等 TTL |
| 审计盲区 | 所有 ephemeral policy 生命周期写入 AuditLog |

### 9.5 GC Controller 失败处理

GC Controller 挂掉时，过期 Policy 若不删除，等于权限一直保留——这是 Ephemeral Access 的核心风险点。需要**双重防御**：

| 防线 | 机制 | 说明 |
|------|------|------|
| **防线一：GC Controller**（软删除）| reconcile loop 每分钟扫描并删除过期 Policy | 正常路径 |
| **防线二：eBPF 数据面 TTL**（硬截止）| eBPF 程序在 `tc_ingress.bpf.c` 中读取 Policy 的 `expiresAt`，到期后直接在数据包级别拒绝，不等控制面删除 | GC Controller 故障时的兜底 |
| **防线三：Lattice Agent 心跳**| Lattice Agent 定期向控制面确认 Ephemeral Policy 仍有效，无响应时本地撤销 | 控制面完全不可用时的兜底 |

**结论**：即使 GC Controller 完全宕机，防线二和三保证 Ephemeral 通道在数据面仍然到期关闭。

### 9.6 实现代价评估

| 组件 | 工作量 | 说明 |
|------|--------|------|
| `LatticePolicy` 加 `expiresAt` 字段 | 小 | CRD 加字段 + validation webhook |
| GC Controller（watch 过期 + 删除）| 小 | 标准 controller-runtime reconcile loop |
| eBPF 数据面 TTL 检查 | 中 | 扩展 `tc_ingress.bpf.c`，读取 expiresAt 做时间比较 |
| Agent Access Request API | 中 | 新 endpoint，复用现有 IntentService |
| 低风险自动审批规则引擎 | 中 | 从简单 heuristic 开始，逐步完善 |
| eBPF 规则热更新 | 已有 | PolicyEnforcer 删 policy 即撤规则，无需改造 |

**总体评估**：80% 依赖现有基础设施，是方案 B 里代价最小、安全收益最大的功能点之一。

### 9.7 在优先级表中的位置

| 技术 | 优先级 |
|------|--------|
| `LatticePolicy.expiresAt` + GC Controller | P1 |
| Agent Access Request API | P1 |
| 低风险自动审批规则 | P2 |
| 人工审批路径（复用 WorkflowService）| P1（已有，直接复用）|

---

## 十、Agent 运行形态与零代码改动保护

### 10.1 Agent 的运行形态

Agent **不一定是容器**。现实中有五种形态：

```
1. 裸进程        python agent.py 跑在一台 VM 或开发者笔记本上
2. Docker 容器   docker run my-agent
3. K8s Pod       由 K8s 调度管理的容器
4. Serverless    Lambda / Cloud Function，短生命周期
5. MicroVM       Firecracker 等，有独立内核的轻量虚拟机
```

**今天绝大多数 AI Agent 的实际形态是裸进程**——一个跑着 LangChain / AutoGen / Claude SDK 的 Python 脚本，部署在某台机器上，没有任何容器化。

### 10.2 不改代码如何保护：下沉到 Agent 看不见的层

核心思路：**把隔离下沉到 Agent 看不见、也改不了的层**。

#### 层一：网络层（WireGuard Overlay）

WireGuard 是内核模块，在操作系统层创建虚拟网络接口（`wf0`）。Agent 进程完全不知道它的存在。

```
Agent 进程发出 HTTP 请求
    ↓
操作系统路由表（由 Lattice 注入）
    ↓
wf0 接口（WireGuard 加密隧道）
    ↓
LatticePolicy 检查：这个目标允许吗？
    ↓ 允许                  ↓ 拒绝
  转发出去              数据包丢弃，Agent 无感知
```

Agent 的代码不需要任何修改。它只是在调用普通网络 API，路由层已经被 Lattice 接管。

#### 层二：eBPF（内核钩子）

eBPF 程序挂载在内核的 tracepoint / TC hook 上，监控所有流量和系统调用。这个监控对 Agent **完全不可见**——Agent 无法知道自己被监控，也无法关闭它。

```
Agent 进程尝试：
  read("/etc/ssh/id_rsa")  → eBPF 检测到，阻断 + 告警
  connect("10.0.0.5:22")   → eBPF 查 LatticePolicy，未授权则丢弃
  fork() 创建子进程         → eBPF 记录，触发异常检测
```

Lattice 已有此基础：`internal/agent/ebpf/tc_ingress.bpf.c` 挂在 `wf0` 接口上，当前做策略执行，后续可扩展到行为监控。

#### 层三：进程沙箱（OS 级，对容器和裸进程均适用）

```
Linux Namespace   → 隔离网络、文件系统、进程视图（Agent 看不到其他进程）
cgroups           → 限制 CPU / 内存，防止单个 Agent 耗尽资源
seccomp profile   → 白名单可用系统调用（禁止 execve、mount 等危险调用）
```

这些是 Linux 内核原语，在 Agent 进程启动时由宿主机 / 容器运行时注入，无需修改 Agent 代码。

### 10.3 各形态下 Lattice 的接入方式

| Agent 运行形态 | Lattice 接入方式 | 代码改动 |
|---------------|----------------|---------|
| **裸进程（VM / 笔记本）** | 同机安装 Lattice Agent，注入路由规则，eBPF 挂载到网卡 | 零 |
| **Docker 容器** | 宿主机运行 Lattice Agent 共享 host network；或以 sidecar 容器接入 | 零 |
| **K8s Pod** | Lattice K8s Operator（已有 `cmd/manager`）自动注入 init container 配置路由 | 零 |
| **Serverless** | Agent 通过 Lattice Relay（LRP）作为出口代理，策略在 LRP 层执行 | 零（仅配置出口代理）|
| **MicroVM** | Lattice Agent 运行在 MicroVM 内部，WireGuard 建立加密隧道出去 | 零 |

### 10.4 保护层次图

```
┌─────────────────────────────────────────────┐
│              宿主机 / VM                      │
│                                             │
│  ┌──────────────────┐                       │
│  │   AI Agent 进程   │  ← 完全不知道下面存在  │
│  │  (Python/Go/...)  │                       │
│  └────────┬─────────┘                       │
│           │ 普通网络调用                      │
│  ┌────────▼─────────────────────────────┐   │
│  │           Linux 内核                  │   │
│  │                                      │   │
│  │  eBPF hooks  ← 监控所有 syscall/流量  │   │
│  │       ↓                              │   │
│  │  路由表（Lattice 注入）               │   │
│  │       ↓                              │   │
│  │  wf0（WireGuard 接口）               │   │
│  │       ↓                              │   │
│  │  LatticePolicy 检查                  │   │
│  └──────────────────────────────────────┘   │
│                    ↓ 允许的流量               │
│              加密隧道出去                     │
└─────────────────────────────────────────────┘
```

Agent 只能看到最上层。它以为自己在直接访问网络，实际上所有流量在内核层已被 Lattice 完全接管。

---

## 十一、竞争定位：为什么不用 K8s Network Policy

当被问到"K8s 的 Network Policy 不够用吗"时，回答如下：

| 维度 | K8s Network Policy | Lattice Agent 隔离 |
|------|-------------------|-------------------|
| **管控粒度** | 容器 / Pod 级别 | **Agent 实例 / 会话级别** |
| **身份模型** | IP + Label | **加密密钥对（NHI），不可伪造** |
| **运行环境** | 仅 K8s 集群内 | **跨云、笔记本、移动端、边缘节点** |
| **权限模型** | 静态规则，手动维护 | **意图驱动，动态 Ephemeral，自动销毁** |
| **监控深度** | 无（依赖 CNI 插件）| **eBPF 内核层，无侵入，Agent 无法绕过** |
| **Agent 代码改动** | 应用需感知网络策略 | **零改动，完全透明** |

**核心差异**：K8s 管的是"容器"，Lattice 管的是"Agent 这个身份在这次任务里能做什么"。这是粒度、身份模型和覆盖范围三个维度的同时升级。

---

## 十二、完整实现清单：AgentIdentity 不够，还差什么

### 12.1 AgentIdentity CRD 的边界

`AgentIdentity` 只是一个数据模型——存储了身份和权限配置，但没有任何执行层真正使用它。完整实现需要在它之上再建五层。

### 12.2 缺口分析

#### 缺口一：身份生命周期（谁来颁发身份？）

```
Agent 启动
  → 谁来给它生成 WireGuard 密钥对？
  → 谁来签发 JWT？JWT 里带什么字段？
  → 密钥怎么安全地传给 Agent？
  → Agent 离线或注销时谁来清理？
```

**Bootstrap 问题（先有鸡还是先有蛋）**：Agent 第一次注册时还没有任何身份，如何证明自己？

解法参考 K8s Bootstrap Token 模式：

```
1. 管理员在控制面生成一次性 Enrollment Token（短期有效，如 1 小时）
   POST /api/v1/agents/enrollment-tokens
   → { "token": "abc123", "expiresAt": "...", "allowedNamespaces": ["ws-a"] }

2. 管理员将 Token 以安全方式传递给 Agent（环境变量、Secret、配置文件）

3. Agent 首次启动，携带 Token 发起注册
   POST /api/v1/agents
   Authorization: Bearer <enrollment-token>
   { "name": "agent-monitoring-prod", "publicKey": "<wg-pubkey>" }

4. LatticeD 验证 Token 有效性：
   → 创建 AgentIdentity CRD
   → 创建 LatticePeer（绑定公钥）
   → 签发长期 Agent JWT
   → Token 立即作废（单次使用）

5. 后续所有请求使用 Agent JWT，不再需要 Enrollment Token
```

**JWT Payload 字段**：
```json
{
  "sub": "agent-monitoring-prod",
  "agent_id": "agent-monitoring-prod",
  "namespace": "workspace-a",
  "allowed_tools": ["list_peers", "list_policies"],
  "iat": 1746748800,
  "exp": 1778284800
}
```

需要新增 `AgentRegistrationService` + 注册 API + Enrollment Token 管理。

#### 缺口二：控制面执行点（谁来用 AgentIdentity 执行规则？）

现在 `ExecuteTool()` 完全没有检查调用者身份：

```go
// 现在：直接执行，无任何身份校验
func (s *aiService) ExecuteTool(ctx, namespace, name string, input json.RawMessage) (string, error) {
    switch name { ... }
}

// 需要变成：
func (s *aiService) ExecuteTool(ctx, namespace, name string, input json.RawMessage) (string, error) {
    agentID := extractAgentID(ctx)              // 从 JWT 提取
    identity := loadAgentIdentity(agentID)      // 查 AgentIdentity CRD
    if !identity.AllowedTools[name] {           // 检查工具白名单
        return "", ErrForbidden
    }
    if !identity.AllowedNamespaces[namespace] { // 检查 namespace 归属
        return "", ErrForbidden
    }
    // 然后才执行
}
```

需要新增 JWT middleware + 改造 `ExecuteTool()`。

#### 缺口三：数据面绑定（裸进程如何区分哪个 Agent？）

在一台裸机上跑多个 AI Agent 进程，eBPF / WireGuard 怎么知道哪个流量属于哪个 Agent？

目前 Lattice Agent（`cmd/lattice`）管的是**整台机器**的网络，不区分进程。

三个选项：

| 选项 | 做法 | 优劣 |
|------|------|------|
| **A：独立 network namespace** | 每个 AI Agent 对应一个 Lattice Agent 实例，各自管理独立 network namespace + WireGuard 接口 | 实现简单，资源开销大 |
| **B：cgroup 标记（推荐）** | 给每个 AI Agent 进程分配唯一 cgroup，eBPF 通过 cgroup ID 识别进程归属 | 粒度精确，需 eBPF 程序扩展 |
| **C：强制容器化** | AI Agent 必须跑在容器里，容器 ID 映射 AgentIdentity | 实现最简单，牺牲裸进程场景 |

社区版走选项 C（容器化），Pro 版走选项 B（cgroup + eBPF）。

#### 缺口四：Ephemeral Access 执行层

| 需要的组件 | 现状 |
|-----------|------|
| `LatticePolicy.spec.expiresAt` 字段 | CRD 里没有 |
| GC Controller（watch 过期 policy 并删除）| 没有 |
| Agent Access Request API | 没有 |
| 低风险自动审批规则引擎 | 没有 |
| 主动 revoke 接口 | 没有 |

#### 缺口五：审计管道

`audit.go` 存在，但只记录用户操作，不记录 AI Agent 的工具调用。缺失的审计事件：

```
agent-x 在 14:30 调用了 list_peers                    → 未记录
agent-x 在 14:31 尝试调用 delete_peer，被 RBAC 拒绝   → 未记录
agent-x 在 14:35 发起 Ephemeral Access 申请           → 未记录
ephemeral-policy-xxx 在 15:00 TTL 到期，自动销毁       → 未记录
```

### 12.3 完整组件清单

#### 新增 CRD / CRD 字段

| 组件 | 类型 | 说明 |
|------|------|------|
| `AgentIdentity` | 新增 CRD | 身份 + 权限数据模型 |
| `LatticePolicy.spec.expiresAt` | 改动现有 CRD | Ephemeral Access TTL 字段 |

#### 新增 API

| API | 说明 |
|-----|------|
| `POST /api/v1/agents` | Agent 注册，颁发密钥 + JWT |
| `DELETE /api/v1/agents/{id}` | Agent 注销，清理密钥和 Policy |
| `POST /api/v1/agents/{id}/access-request` | JIT 访问申请 |
| `DELETE /api/v1/agents/{id}/access/{policy-id}` | 主动 revoke ephemeral policy |
| `GET /api/v1/agents/{id}/audit` | 查询 Agent 操作审计 |

#### 新增 Service / Controller

| 组件 | 说明 |
|------|------|
| `AgentRegistrationService` | 生成密钥对、签发 JWT、创建 AgentIdentity + LatticePeer |
| `AgentGCController` | watch `LatticePolicy.expiresAt`，到期自动删除 |
| `AgentAccessRequestService` | 包装 IntentService，处理 JIT 申请 + 自动审批规则 |

#### 改造现有组件

| 组件 | 改动内容 |
|------|---------|
| `ExecuteTool()` in `ai.go` | 加 AgentIdentity RBAC 校验（工具白名单 + namespace 归属）|
| HTTP middleware | 加 Agent JWT 解析和验证 |
| `audit.go` | 扩展记录工具调用、RBAC 拒绝、Ephemeral 事件 |
| `cmd/lattice` Lattice Agent | 支持进程/容器-身份绑定（社区版容器化，Pro 版 cgroup）|

#### Pro 路线图（暂缓）

| 组件 | 说明 |
|------|------|
| eBPF cgroup 进程识别 | 裸进程场景的精确身份绑定 |
| eBPF 行为监控扩展 | syscall 白名单、异常检测告警 |
| Firecracker MicroVM 托管 | 代码执行 Agent 的内核级隔离 |
| 异常熔断器 | Agent 行为异常时自动网络隔离 |

### 12.4 工作量估算

| 模块 | 估算 | 备注 |
|------|------|------|
| `AgentIdentity` CRD + webhook | 小 | 标准 kubebuilder 脚手架 |
| `LatticePolicy.expiresAt` + GC Controller | 小 | 标准 reconcile loop |
| `AgentRegistrationService` + 注册 API | 中 | 密钥生成 + JWT 签发 |
| JWT middleware + `ExecuteTool()` RBAC | 中 | 改造现有热路径，需仔细测试 |
| `AgentAccessRequestService` + JIT API | 中 | 复用 IntentService，新增规则引擎 |
| `audit.go` 扩展 | 小 | 扩展现有结构 |
| 容器-身份绑定（社区版）| 中 | K8s label 映射 AgentIdentity |
| **合计（社区版 MVP）** | **约 6-8 周** | 不含 Pro 路线图 |

---

## 十三、配置化开关设计：与现有安全网络的集成

### 13.1 Lattice 现有的三种开关模式

代码里已有三种成熟模式，Agent 隔离直接复用，不引入新范式：

**模式一：编译期 Build Tag（Pro vs 社区版）**
```go
//go:build pro
// 只在 EDITION=pro 时编译进去，社区版二进制里不存在
```

**模式二：运行时 nil 注入（已用于 intentSvc、snapStore）**
```go
// 关闭时传入 nil，调用时做 nil check 自动降级
if s.intentSvc == nil {
    return "", fmt.Errorf("... is a Pro feature")
}
```

**模式三：配置文件开关（已用于 ai.enabled、autoApprove）**
```go
if !cfg.AI.Enabled {
    return nil, fmt.Errorf("ai is not enabled")
}
```

Agent 隔离使用**模式二 + 模式三组合**：配置决定是否注入，nil check 决定行为。

### 13.2 三层配置结构

#### 第一层：全局开关（`lattice.yaml`）

```yaml
ai:
  enabled: true
  agent-isolation:
    enabled: true               # 总开关
    enforcement-mode: enforce   # disabled | audit | enforce
    ephemeral-access: true      # Ephemeral Access 子功能开关
    audit-level: full           # none | write | full
```

三种 enforcement-mode 的行为差异：

| 模式 | AgentIdentity 校验 | 违规时行为 | 用途 |
|------|-------------------|-----------|------|
| `disabled` | 不校验 | — | 完全关闭，等同当前行为 |
| `audit` | 校验但不阻断 | 只写日志 | 灰度上线，观察影响 |
| `enforce` | 强制校验 | 拒绝请求 | 正式开启 |

`audit` 模式是关键：先开启观察，确认没有误伤再切 `enforce`，无需停服。

#### 第二层：工作区级覆盖（LatticeNetwork annotation）

```yaml
kind: LatticeNetwork
metadata:
  annotations:
    lattice.io/agent-isolation: "strict"   # strict | permissive | disabled
```

允许某个工作区比全局更严格或豁免，不影响其他工作区。

#### 第三层：单 Agent 级（AgentIdentity spec）

```yaml
kind: AgentIdentity
spec:
  enforcementMode: enforce   # 可覆盖工作区设置
  auditLevel: full
```

### 13.3 与现有安全网络的关系：分层叠加

**Agent 隔离是叠加层，不是替代层。**

```
Layer 0: WireGuard 加密        → 始终开启，不可关闭
Layer 1: LatticePolicy 零信任  → 始终开启，不可关闭
─────────────────────────────────── ← 现有安全网络边界
Layer 2: AgentIdentity + RBAC  → 可配置，默认关闭
Layer 3: Ephemeral Access      → 可配置，默认关闭
Layer 4: eBPF 行为监控         → Pro，可配置，默认关闭
```

**关闭 Agent 隔离 ≠ 不安全。** WireGuard + LatticePolicy 始终运行，关闭只是移除 Agent 粒度的精细控制，回到"所有认证用户共享工具集"的当前状态。网络层安全不受影响。

### 13.4 代码层面的优雅开关

复用现有 nil 注入模式，`AgentIsolationService` 为 nil 时自动降级，原有逻辑完全不改：

```go
// internal/server/service/ai.go

func (s *aiService) ExecuteTool(ctx context.Context, namespace, name string, input json.RawMessage) (string, error) {

    // Agent 隔离开关：nil 时跳过，行为与现在完全一致
    if s.agentIsolation != nil {
        if err := s.agentIsolation.CheckToolAccess(ctx, namespace, name); err != nil {
            return "", err  // enforce 模式：拒绝；audit 模式：只记日志后继续
        }
    }

    switch name { ... } // 原有逻辑不动
}
```

启动时根据配置决定是否注入：

```go
// cmd/latticed/main.go

var agentIsolation service.AgentIsolationService // 默认 nil = 关闭
if cfg.AI.AgentIsolation.Enabled {
    agentIsolation = service.NewAgentIsolationService(cfg, store, k8s)
}

aiSvc := service.NewAIServiceWithWorkflow(
    llmClient, store, k8s, presence, maxToolCalls,
    workflowSvc, autoApprove,
    agentIsolation,  // nil 时所有校验自动跳过
)
```

### 13.5 上线路径（灰度策略）

```
第一步：disabled（默认，部署即生效）
  → 部署新代码，所有行为与之前完全一致
  → 用户无感知，零风险

第二步：audit（开灯观察）
  → 切换 enforcement-mode: audit
  → 观察日志：哪些调用没有 AgentIdentity？影响范围多大？
  → 不阻断任何请求，业务无影响

第三步：enforce（逐步收紧）
  → 先在单个测试工作区开启：lattice.io/agent-isolation: strict
  → 确认无误扩大范围，最终升级全局配置
  → 正式生效
```

---

## 十四、下一步（按优先级排序）

**P0 — 身份与控制面（地基）**
1. 细化 `AgentIdentity` CRD 字段，生成 kubebuilder 脚手架
2. 实现 `AgentRegistrationService`：WireGuard 密钥对生成 + JWT 签发
3. 新增注册 API：`POST /api/v1/agents`
4. HTTP middleware 加 Agent JWT 验证
5. 改造 `ExecuteTool()` 加工具白名单 + namespace RBAC 校验
6. 扩展 `audit.go` 记录工具调用和 RBAC 拒绝事件

**P1 — Ephemeral Access（核心安全能力）**
7. `LatticePolicy` 加 `expiresAt` 字段 + CRD validation webhook
8. 实现 `AgentGCController`：watch 过期 policy 自动删除
9. 实现 `AgentAccessRequestService` + JIT API：`POST /api/v1/agents/{id}/access-request`
10. 主动 revoke 接口：`DELETE /api/v1/agents/{id}/access/{policy-id}`

**P1 — 数据面绑定（社区版）**
11. 容器化 Agent 身份绑定：K8s label → AgentIdentity 映射
12. 实现 Pod sandbox 托管（K8s Pod + seccomp profile）

**P2 — Pro 路线图**
13. eBPF cgroup 进程识别（裸进程身份绑定）
14. eBPF 行为监控扩展（syscall 白名单 + 异常检测）
15. Firecracker MicroVM 托管
16. 低风险 Ephemeral Access 自动审批规则引擎
