# Home 页重设计与 Agent 沙箱 Dashboard

> 状态: 设计阶段 | 关联: `2026-05-11-agent-sandbox-and-ecosystem-design.md` `2026-05-12-agent-sandbox-usage.md`

## 概述

以 AI Agent 安全沙箱为产品新主线重写 Home landing page (pages/index.vue)，并在 Dashboard 中增加 Agent 沙箱管理页面组（沙箱列表、接入令牌、流量审计）。

## 定位

- **Home 页**: Marketing landing page，双主线叙事——"AI Agent 运行时" + "安全网络基础层"
- **Dashboard**: 新增 Agent 沙箱管理分组，Dashboard 首页增加沙箱统计卡片

---

## 一、Home 页重设计

### 1.1 Hero 区

**Badge**: `AI-Native Networking`（脉冲绿点保留）

**Title**: "AI Agent 的零信任运行环境"

**Subtitle**: "Lattice 为每个 AI Agent 提供零特权沙箱隔离，通过 WireGuard 加密 Mesh 安全接入你的基础设施——从启动到组网，一条命令。"

**CTA 双按钮**:
- 主按钮: `lattice-agent-sandbox start` → `/manage/stepper`
- 次按钮: `查看控制台` → `/dashboard`

**Terminal mockup** 展示沙箱启动和流量管控流程:

```
$ lattice-agent-sandbox start --name my-agent
  → Creating gVisor sandbox...      ✓
  → Attaching WireGuard tunnel...   ✓
  → Registering with control plane.. ✓
  Agent "my-agent" is online at 10.100.0.5

# 合法外联 (白名单命中)
$ curl https://api.internal
  → PolicyCache.Allow() → ✓ → wireguard-go encrypt
  → P2P/Relay → 远端解密 → 200 OK

# 违规外联 (未命中)
$ curl https://evil.com
  → PolicyCache.Allow() → ✗ → EACCES
  → auditCh → POST /api/v1/audit/batch → 控制面告警
```

**视觉效果**: 保留后台 SVG 网络拓扑图 + 紫色 glow。终端窗口风格与当前一致（深色背景、macOS 红黄绿按钮、monospace 字体）。

### 1.2 Features 区

双行 3 列网格。上行 AI Agent 能力，下行网络基础能力。

| 行 | 卡片 | 标题 | 描述 | 标签 |
|----|------|------|------|------|
| AI | 1 | 零特权沙箱隔离 | gVisor 用户态内核运行 Agent，无需 root/CAP_NET_ADMIN。每个 Agent 拥有独立 Go netstack，PID→TUN 绑定，外联全管控 | `stable` |
| AI | 2 | Sidecar 外联拦截 | seccomp notify + eBPF fast path 双重拦截。Agent 每次 socket 操作向 PolicyCache 查表，白名单命中直通，未命中→EACCES+审计 | `stable` |
| AI | 3 | 意图驱动网络管理 | 用自然语言描述网络需求——"allow frontend to access api on 8080"——AI 自动生成变更计划，Diff 预览后通过 RBAC 审批生效 | `roadmap` |
| 网络 | 4 | WireGuard 加密 Mesh | NAT 穿透 P2P 直连 + LRP Relay 中继。WireGuard 端到端加密，ICE/STUN 自动打洞，跨集群桥接零配置 | `stable` |
| 网络 | 5 | eBPF 高性能策略 | 内核态 LPM Trie + Port Hash 策略引擎，TC ingress/egress 挂载，百万规则级匹配。基础设施层东西向流量线速转发 | `roadmap` |
| 网络 | 6 | 全局流量审计 | gVisor Go 层 + eBPF ring buffer 双路径汇聚，批量上报控制面 `POST /api/v1/audit/batch`，异常模式实时告警 | `stable` |

**i18n key 模式**: `landing.features.ai_sandbox`, `landing.features.ai_sidecar`, `landing.features.ai_intent`, `landing.features.net_wg`, `landing.features.net_ebpf`, `landing.features.net_audit`

### 1.3 Architecture 区

双栏布局：左栏 Agent 沙箱生命周期步骤，右栏数据面 Terminal mockup。

**左栏：Agent 沙箱生命周期**

```
01  创建沙箱
    lattice-agent-sandbox start --name agent-1
    → 生成 WireGuard 密钥对
    → 创建 gVisor sandbox (OCI runtime)
    → Go netstack 接管网络栈

02  注册 + 策略注入
    → POST /api/v1/agent-isolation/register
    → 控制面创建 LatticePeer + AgentIdentity (Sandbox=gvisor)
    → 注入 PolicyChecker (白名单) + AuditWriter (审计 hook)

03  Agent 运行
    → Agent 进程发起 socket → Sentry netstack 拦截
    → PolicyCache.Allow() 查表
    → 命中(allow): wireguard-go 加密 → P2P/Relay 发送 → 远端解密
    → 未命中(drop): EACCES 返回调用方 + audit event 上报
```

**右栏：Terminal mockup（数据面路径）**

```
# 合法流量 (allow verdict path)
agent-1 socket → Sentry netstack → Allow
  → wireguard-go ChaCha20 encrypt
  → UDP :51820 → FilteringUDPMux
  → ICE P2P / LRP Relay → 远端 peer → decrypt → target

# 违规流量 (drop verdict path)
agent-1 socket → Sentry netstack → Deny → EACCES
  → ns.auditCh → AuditBatcher
  → POST /api/v1/audit/batch → 控制面告警
```

**i18n key 模式**: `landing.architecture.sandbox_step_1_title/desc`, etc.

### 1.4 Pricing 区

保留 Community / Pro 双栏布局。Feature 列表根据三层 Agent 沙箱能力更新。

**Community 栏**:

| # | Feature |
|---|---------|
| 1 | WireGuard 加密 Mesh：无限节点 |
| 2 | CRD 原生 K8s 控制器 |
| 3 | NATS 信令 + ICE/STUN P2P 打洞 |
| 4 | LRP Relay 中继 |
| 5 | Agent 沙箱 (cgroup 模式)：PID 绑定 + 资源限制 |
| 6 | Sidecar 外联拦截：seccomp + eBPF fast path |
| 7 | 全局拓扑图可视化 |
| ~~8~~ | (locked) gVisor 零特权沙箱隔离 |
| ~~9~~ | (locked) eBPF 流量审计 + 异常告警 |
| ~~10~~ | (locked) 意图引擎 (自然语言网络管理) |

**Pro 栏**:

| # | Feature |
|---|---------|
| 1 | 所有社区版功能 |
| 2 | gVisor 零特权沙箱隔离：Go netstack 替代 TUN 设备 |
| 3 | eBPF TC 策略引擎 (LPM/Port) + 流量镜像审计 |
| 4 | 意图引擎：自然语言网络变更计划 |
| 5 | 合规审计扫描 + 异常告警 |
| 6 | Kubernetes 集群互联 |
| 7 | SSO/OIDC + RBAC + 审批工作流 |
| 8 | Firecracker MicroVM 沙箱 (远期增强) |

**i18n key 模式**: `landing.pricing.community_feat_sandbox`, `landing.pricing.pro_feat_gvisor`, etc.

### 1.5 CTA 区

- **Title**: "为你的 AI Agent 构建安全运行环境"
- **主按钮**: `$ lattice-agent-sandbox start` → `/manage/stepper`
- **次按钮**: `查看控制台` → `/dashboard`
- **Bottom badges**: 零特权 / gVisor 隔离 / WireGuard 加密 / cgroup 限制 / Sidecar 拦截 / 流量审计

### 1.6 Footer

保持现有结构不变（logo + copyright + 链接）。

---

## 二、Dashboard 新增内容

### 2.1 侧边栏

在 "AI Assistant" 分组后新增 "Agent 沙箱" 分组:

```
┌─────────────────────────────┐
│ 🛡️ Agent 沙箱               │
│   沙箱列表    /sandbox       │
│   接入令牌    /sandbox/tokens │
│   流量审计    /sandbox/audit  │
└─────────────────────────────┘
```

**i18n key**: `common.nav.group.sandbox`, `common.nav.sandboxList`, `common.nav.sandboxTokens`, `common.nav.sandboxAudit`

### 2.2 Dashboard 首页

在现有 4 张统计卡片同行新增第 5 张：

| 卡片 | 指标 | 图标 | 颜色 |
|------|------|------|------|
| Active Sandboxes | 运行中沙箱数 (在线/total)，sparkline | `Bot` 或 `Container` | violet |

**数据来源**: `GET /api/v1/agent-isolation/agents` 聚合统计。

### 2.3 新增页面：沙箱列表 `/sandbox`

**表格列**: 名称, 状态(online/offline), 隔离模式(gVisor/cgroup), VPN IP, Sandbox ID, 流量 Rx/Tx, 创建时间, 操作(吊销)

**操作**: 吊销按钮触发 `DELETE /api/v1/agent-isolation/agents/:name`，弹出确认框。

**状态颜色**: online → emerald, offline → muted

**i18n key**: `sandbox.list.title`, `sandbox.list.desc`, `sandbox.list.col*`, `sandbox.list.revoke`, `sandbox.list.confirmRevoke`

### 2.4 新增页面：接入令牌 `/sandbox/tokens`

**上半部分**: 创建令牌表单

| 字段 | 类型 | 说明 |
|------|------|------|
| TTL | select/number | 1h / 6h / 24h / Custom |
| Allowed Tools | multi-select checkbox | list_peers, list_policies, check_connectivity, ... |

创建按钮调用 `POST /api/v1/agent-isolation/enrollment-tokens`，成功后展示一次性 token（复制按钮 + 警告"仅此一次可见"）。

**下半部分**: 令牌列表表格

| 列 | 说明 |
|----|------|
| Token (masked) | `a1b2c3...xxxx` |
| 创建时间 | timestamp |
| 过期时间 | timestamp |
| 状态 | active / expired / revoked |
| 操作 | 撤销 (DELETE) |

### 2.5 新增页面：流量审计 `/sandbox/audit`

**内容**: Agent 流量事件表，来自 `GET /api/v1/audit?type=traffic`

| 列 | 说明 |
|----|------|
| 时间 | timestamp |
| 沙箱名称 | identity/sandbox name |
| 源 IP | src_ip |
| 目标 IP:Port | dst_ip:dst_port |
| 协议 | tcp/udp |
| Verdict | allow (emerald) / drop (rose) |
| 详情 | 展开查看完整 event JSON |

**过滤**: 搜索框（按沙箱名/dst_ip），verdict 下拉过滤（allow/drop/all），时间范围选择。

**i18n key**: `sandbox.audit.title`, `sandbox.audit.desc`, `sandbox.audit.col*`, `sandbox.audit.filter*`

---

## 三、Routes 汇总

| 路由 | 页面 | meta layout |
|------|------|-------------|
| `/` | pages/index.vue (Home) | `blank` (不变) |
| `/dashboard` | pages/dashboard/index.vue | `default` (不变) |
| `/sandbox` | pages/sandbox/index.vue (新增) | `default` |
| `/sandbox/tokens` | pages/sandbox/tokens.vue (新增) | `default` |
| `/sandbox/audit` | pages/sandbox/audit.vue (新增) | `default` |

---

## 四、文件变更清单

### 修改文件

| 文件 | 变更内容 |
|------|----------|
| `fronted/src/pages/index.vue` | 按本文档第一节重写全部区块 |
| `fronted/src/locales/en/landing.json` | 更新/新增所有 landing i18n keys |
| `fronted/src/locales/zh-CN/landing.json` | 同上 |
| `fronted/src/locales/en/common.json` | 新增 sandbox 相关 i18n keys |
| `fronted/src/locales/zh-CN/common.json` | 同上 |
| `fronted/src/components/app-sidebar/AppSidebar.vue` | 新增 Agent 沙箱导航分组 |
| `fronted/src/pages/dashboard/index.vue` | 新增第 5 张统计卡片 (Active Sandboxes) |

### 新增文件

| 文件 | 说明 |
|------|------|
| `fronted/src/pages/sandbox/index.vue` | 沙箱列表页 |
| `fronted/src/pages/sandbox/tokens.vue` | 接入令牌管理页 |
| `fronted/src/pages/sandbox/audit.vue` | Agent 流量审计页 |
| `fronted/src/stores/useSandboxStore.ts` | 沙箱相关状态管理 |

---

## 五、实现顺序

1. **i18n keys**: 先补全所有 `landing.*` 和 `sandbox.*` 的 i18n keys (中英文)
2. **Sidebar**: 新增 Agent 沙箱导航分组
3. **Home 页重写**: 按区块顺序实现 (Hero → Features → Architecture → Pricing → CTA → Footer)
4. **Dashboard 更新**: 新增第 5 张卡片 + store 扩展
5. **新增页面**: 沙箱列表 → 接入令牌 → 流量审计 (顺序依依赖)
