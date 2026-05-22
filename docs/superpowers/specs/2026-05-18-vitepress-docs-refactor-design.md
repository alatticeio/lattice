# VitePress Docs Refactor Design

> 日期: 2026-05-18
> 范围: `docs/` 目录下的 VitePress 文档站点
> 目标: 基于最新 Agent Platform 实现重构主页与文档结构

---

## 背景

当前文档存在以下问题：

1. **主页 features 卡片**未体现 Agent Platform（gVisor sandbox）这一新能力
2. **`/agent/` 路径不存在**，sandbox 相关内容散落在 `ai/agent-enrollment.md`
3. **designSidebar()** 链接到不存在的 `/design/` 路径，导致死链
4. **`ai/agent-enrollment.md`** 混合了 HTTP API 与 CLI sandbox 两种不同的接入方式
5. **Feature Map 表格**缺少 Agent Sandbox、Sub-agent Delegate API、MCP Tool Tracing 等新功能行

---

## 设计决策

### 主页布局

保持 VitePress 默认 `layout: home`（方案 A），不引入自定义 Vue 组件。变更点：

- `hero.text` 改为 `"WireGuard Overlay Network for AI Workloads"`
- `hero.tagline` 改为 `"Zero-privilege agent sandbox · Kubernetes-native · Open-core"`
- 第二个 action 按钮从 "Live Demo" 改为 "Agent Platform"，链接 `/agent/`
- features 列表：第一项改为 Agent Sandbox（gVisor），其余调整描述文字
- Feature Map 新增 4 行：Agent Sandbox / Sub-agent Delegate API / MCP Tool Tracing / NATS Flow Audit

### 导航栏

```
Docs | Deploy | Agent(新增) | AI | Blog | Compare | GitHub
```

删除 "Console" 外链（移至 footer message）。

### 侧边栏拆分

| 路径前缀 | 侧边栏函数 | 状态 |
|---------|----------|------|
| `/guide/` `/deploy/` `/config/` `/features/` `/faq/` | `userSidebar()` | 保持，移除 AI 小节 |
| `/agent/` | `agentSidebar()`（新增） | 新增 |
| `/ai/` | `aiSidebar()`（从 userSidebar 拆出） | 拆分 |
| `/design/` `/adr/` | `designSidebar()`（修复路径） | 修复 |

### Agent Platform 侧边栏（agentSidebar）

```
Agent Platform
  ├── Overview              → /agent/
  ├── Sandbox (Community)   → /agent/sandbox
  ├── Sandbox (Pro)         → /agent/sandbox-pro
  └── Sub-agent Delegate API → /agent/delegate-api
```

### Design 侧边栏（修复后）

```
Architecture
  ├── Overview              → /design/architecture
  ├── Sandbox Architecture  → /design/sandbox
  ├── ICE Connection        → /design/ice-connection
  ├── ICE + WireGuard Mux   → /design/ice-wireguard-mux
  └── WRRP / QUIC           → /design/wrrp-quic
ADR
  └── 0001 - Performance Benchmark → /adr/0001-...（已存在）
```

移除 Development 小节（build/ci 文件不存在）。

---

## 文件变更清单

### 修改（6 个）

| 文件 | 主要变更 |
|------|---------|
| `index.md` | hero text/tagline/actions、features 卡片、Feature Map 表格 |
| `.vitepress/config.mts` | 新增 agent 导航项、agentSidebar()、aiSidebar()、修复 designSidebar() |
| `ai/index.md` | Layer 2 描述更新：明确 gVisor sandbox 是 CLI 方式，HTTP API 是另一路径 |
| `ai/agent-enrollment.md` | 页面顶部加说明框区分 HTTP API vs CLI sandbox，加跳转链接 |
| `guide/quickstart.md` | 更新版本号引用，确保命令与当前实现一致 |
| `.vitepress/theme/components/LatticeSandbox.vue` | 版本号 v0.2.0 → 当前版本，sandbox 演示命令更新 |

### 新增（9 个）

**`/agent/` 路径（4 个）**

- `agent/index.md` — Agent Platform 概述：两种接入方式对比表、Community vs Pro 能力矩阵、快速导航
- `agent/sandbox.md` — Community sandbox 完整指南：网络架构图、启动流程（8步）、命令参考、凭证持久化、审计日志、AI 框架集成示例
- `agent/sandbox-pro.md` — Pro 增强功能：EgressFilter（CIDR allowlist）、ForwardListener（overlay 端口转发）、HTTP 正向代理、NATS 流量审计
- `agent/delegate-api.md` — Sub-agent Delegate API：CRD 字段（AgentIdentity.spec.parentRef）、HTTP 端点（POST /api/v1/agents/:id/delegate）、curl + Python 示例

**`/design/` 路径（5 个）**

- `design/architecture.md` — 高层架构概述：三组件表、传输状态机、Policy Enforcement 决策路径
- `design/sandbox.md` — 整理自 `superpowers/specs/2026-05-18-sandbox-agent-architecture.md`
- `design/ice-connection.md` — 整理自 `superpowers/specs/2026-05-01-ice-relay-wireguard-design.md`
- `design/ice-wireguard-mux.md` — 整理自 `superpowers/specs/ice-wireguard-mux.md`
- `design/wrrp-quic.md` — 整理自 `superpowers/specs/design-wrrp-quic.md`

### 不变

所有其他 `features/`、`faq/`、`blog/`、`deploy/`、`config/` 下的页面内容保持不变。

---

## 内容规范

- `/agent/` 页面：英文（与其他文档页一致）
- `/design/` 页面：直接从 specs 整理，保留技术细节，添加页面 frontmatter
- LatticeSandbox.vue 演示命令：改为 `lattice sandbox start` 流程
- 所有页面保持 VitePress 默认主题，不引入新 npm 依赖

---

## 不在范围内

- `style.css` 不做大改（只有 footer 相关的一行）
- 不新增 npm 包
- `faq/ebpf-sandbox.md` 等现有内容页不改
- 不引入 i18n / 多语言
