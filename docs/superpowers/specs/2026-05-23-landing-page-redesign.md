# Landing Page Redesign — Implementation Spec

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有落地页基础上新增三个区块（The Problem、Why Lattice、Open Source Commitment），引入 Google SAIF 框架作为项目背景，完善「为什么做这个项目」的叙事，让页面更专业、更充实。

**Architecture:** 纯前端改动，不涉及 API。新增区块直接插入 `frontend/src/pages/index.vue`，文案统一走 i18n（zh-CN / en 两套 locale），CSS 全部使用现有 design token（`bg-background`、`text-foreground` 等），自动适配明暗模式。

**Tech Stack:** Vue 3.5 · vue-i18n · Tailwind 4 · Lucide Vue

---

## 页面结构（最终）

```
Navbar
Hero（现有）
Terminal Demo（现有）
── NEW ── The Problem（AI 安全危机 + SAIF 映射）
── NEW ── Why Lattice（定位声明 + 三列价值卡片）
Two Pillars（现有）
Features Grid（现有）
Quickstart（现有）
Pricing（现有）
── NEW ── Open Source Commitment（理念 + 统计 + GitHub CTA）
CTA（现有）
Footer（现有）
```

---

## 区块设计详情

### 区块 A：The Problem

**位置：** Terminal Demo 之后，Two Pillars 之前
**section id：** `problem`
**导航栏：** 不加链接（不在主导航中）

**内容结构：**

```
tag-pill: "⚠ AI Security Imperative"
h2: "AI Agent 在没有身份、没有边界的环境中运行"
     EN: "AI Agents Run Without Identity or Boundaries"
subtitle（3句话叙事）:
  - 第1句：传统网络安全工具为静态服务设计，AI Agent 完全不同
  - 第2句：AI Agent 动态启动、跨工具委派、能执行任意代码
  - 第3句：这正是 Google SAIF 所明确警告的攻击面——缺乏隔离的 Agent 是整个基础设施的入口

小标题: "SAIF 要求 → LATTICE 实现"
        EN: "SAIF REQUIREMENT → LATTICE IMPLEMENTATION"

4 张图标卡片（图标 + 标题 + EN 副标题 + 说明）：
  1. 🔒 网络端点安全 / Network & Endpoint Security
     → WireGuard 加密 Mesh，每个 Agent 持有独立密钥对，流量端到端加密

  2. 🛡 供应链攻击防护 / Supply Chain Isolation
     → gVisor 用户态内核，无 root / TUN / iptables，沙箱逃逸无法触达宿主机

  3. 🎯 访问管理 / Access Management
     → Policy 引擎 + AgentIdentity CRD，精细到工具级权限，Sub-agent 权限不超父级

  4. ⚙ 统一平台管控 / Harmonized Platform Controls
     → 单一 K8s-native 控制面，13 个 CRD 统一管理 Agent、隧道、策略全生命周期

底部引用：
  ZH: 参考：Google Secure AI Framework (SAIF) — Mitigate novel AI security risks
  EN: Reference: Google Secure AI Framework (SAIF) — Mitigate novel AI security risks
  链接: https://safety.google/cybersecurity-advancements/saif/
```

**样式规范：**
- 背景：`bg-muted/50 border-y border-border`（与 Two Pillars 交替）
- 卡片：`bg-background border border-border rounded-xl`
- 图标背景色：蓝/绿/紫/黄，使用 Tailwind opacity 变量（如 `bg-blue-500/10`），明暗均可
- 最大宽度：`max-w-4xl mx-auto`

---

### 区块 B：Why Lattice

**位置：** The Problem 之后，Two Pillars 之前
**section id：** `why`

**内容结构：**

```
tag-pill: "💡 Our Approach"
h2: "SAIF 的基础设施层实现，以开源方式交付"
    EN: "The Infrastructure Layer for SAIF, Delivered as Open Source"
subtitle（2句）:
  - 现有网络方案（Calico/Cilium）为静态服务设计，不理解 AI Agent 的工具调用链、动态委派、凭证持久化
  - 我们构建 Lattice，相信每个 AI Agent 都应拥有可审计、可撤销、权限精细的网络身份

3 列价值卡片：
  1. 🔑 身份优先 / Identity-First
     每个 Agent 持有唯一 WireGuard 密钥对，身份不依赖 IP，可撤销、可追溯

  2. 📋 全程可追溯 / Full Observability
     每次 MCP 工具调用自动记录 tool_span（traceId、agentId、耗时、状态），委派链完整可查

  3. 🚫 零特权运行 / Zero Privilege
     gVisor 沙箱，无需 root，一条命令启动，不改变宿主机任何状态
```

**样式规范：**
- 背景：`py-20 px-6`（白/深色交替，与上下区块形成节奏）
- 三列：`grid grid-cols-1 md:grid-cols-3 gap-6`
- 卡片：`bg-muted/50 border border-border rounded-xl p-6`

---

### 区块 C：Open Source Commitment

**位置：** Pricing 之后，CTA 之前
**section id：** `open-source`

**内容结构：**

```
tag-pill: "🌐 Open Source"
h2: "核心能力永远开源，社区优先"
    EN: "Core Capabilities Always Open, Community First"
subtitle（2句）:
  - AI 基础设施不应该是黑盒。WireGuard Mesh、gVisor 沙箱、工具调用追踪——核心安全能力在 Community 版完整开放
  - Apache 2.0 授权，可自托管、可审计、可修改。PRO 版提供企业级扩展，但开源承诺永远不变

4 格统计（2×2 grid，md: 4列）：
  - Apache 2.0    / License
  - Go 1.25       / Language
  - 13 CRDs       / K8s Native
  - Self-hosted   / Zero lock-in

GitHub CTA 按钮：
  ZH: 在 GitHub 上查看源码
  EN: View Source on GitHub
  链接: https://github.com/alatticeio/lattice
副文字: github.com/alatticeio/lattice
```

**样式规范：**
- 背景：`bg-muted/50 border-y border-border py-20 px-6`
- 统计格：`bg-background border border-border rounded-xl text-center p-6`
- 按钮：使用现有 `Button` 组件，`variant="outline"`，带 GitHub SVG 图标

---

## i18n 新增 Key

### `frontend/src/locales/zh-CN/landing.json` 新增

```json
"problem": {
  "tag": "AI 安全威胁",
  "title": "AI Agent 在没有身份、没有边界的环境中运行",
  "subtitle": "传统网络安全工具为静态服务设计：IP 固定、行为可预测、生命周期长。AI Agent 完全不同——它们动态启动、跨工具委派、能够执行任意代码。这正是 Google Secure AI Framework (SAIF) 所明确警告的攻击面：缺乏网络隔离的 AI Agent，一旦被攻破，就是一个对整个基础设施拥有执行权限的入口。",
  "saif_label": "SAIF 要求 → LATTICE 实现",
  "ref_prefix": "参考：",
  "ref_link": "Google Secure AI Framework (SAIF)",
  "ref_suffix": " — Mitigate novel AI security risks",
  "item_1_title": "网络端点安全",
  "item_1_sub": "Network & Endpoint Security",
  "item_1_desc": "WireGuard 加密 Mesh — 每个 Agent 持有独立密钥对，流量端到端加密，控制面与数据面分离",
  "item_2_title": "供应链攻击防护",
  "item_2_sub": "Supply Chain Isolation",
  "item_2_desc": "gVisor 用户态内核 — Agent 在沙箱中运行，无 root、无 TUN、无 iptables，沙箱逃逸无法触达宿主机",
  "item_3_title": "访问管理",
  "item_3_sub": "Access Management",
  "item_3_desc": "Policy 引擎 + AgentIdentity CRD — 精细到工具级的权限控制，Sub-agent 权限不超父级",
  "item_4_title": "统一平台管控",
  "item_4_sub": "Harmonized Platform Controls",
  "item_4_desc": "单一 K8s-native 控制面 — 13 个 CRD 统一管理 Agent、隧道、网络策略的全生命周期"
},
"why": {
  "tag": "我们的方法",
  "title": "SAIF 的基础设施层实现，以开源方式交付",
  "subtitle": "现有的网络方案（Calico、Cilium）为静态服务设计，不理解 AI Agent 的工具调用链、动态委派、凭证持久化。我们构建 Lattice，相信每个 AI Agent 都应拥有可审计、可撤销、权限精细的网络身份。",
  "item_1_title": "身份优先",
  "item_1_desc": "每个 Agent 持有唯一 WireGuard 密钥对，身份不依赖 IP，可撤销、可追溯",
  "item_2_title": "全程可追溯",
  "item_2_desc": "每次 MCP 工具调用自动记录 tool_span（traceId、agentId、耗时、状态），委派链完整可查",
  "item_3_title": "零特权运行",
  "item_3_desc": "gVisor 沙箱，无需 root，一条命令启动，不改变宿主机任何状态"
},
"oss": {
  "tag": "开源",
  "title": "核心能力永远开源，社区优先",
  "subtitle": "AI 基础设施不应该是黑盒。WireGuard Mesh、gVisor 沙箱、工具调用追踪——这些核心安全能力在 Community 版本中完整开放，Apache 2.0 授权，可自托管、可审计、可修改。PRO 版本提供企业级扩展，但开源承诺永远不变。",
  "stat_1_value": "Apache 2.0",
  "stat_1_label": "License",
  "stat_2_value": "Go 1.25",
  "stat_2_label": "Language",
  "stat_3_value": "13 CRDs",
  "stat_3_label": "K8s Native",
  "stat_4_value": "Self-hosted",
  "stat_4_label": "Zero lock-in",
  "cta": "在 GitHub 上查看源码",
  "cta_sub": "github.com/alatticeio/lattice"
}
```

### `frontend/src/locales/en/landing.json` 新增

```json
"problem": {
  "tag": "AI Security Imperative",
  "title": "AI Agents Run Without Identity or Boundaries",
  "subtitle": "Traditional security tools were designed for static services: fixed IPs, predictable behavior, long lifecycles. AI Agents are fundamentally different — they spin up dynamically, delegate across tools, and can execute arbitrary code. This is exactly the attack surface Google's Secure AI Framework (SAIF) warns about: an AI Agent without network isolation is an execution-capable entry point into your entire infrastructure.",
  "saif_label": "SAIF REQUIREMENT → LATTICE IMPLEMENTATION",
  "ref_prefix": "Reference: ",
  "ref_link": "Google Secure AI Framework (SAIF)",
  "ref_suffix": " — Mitigate novel AI security risks",
  "item_1_title": "Network & Endpoint Security",
  "item_1_sub": "网络端点安全",
  "item_1_desc": "WireGuard Encrypted Mesh — every Agent holds its own key pair, traffic is end-to-end encrypted, control plane and data plane are separated",
  "item_2_title": "Supply Chain Isolation",
  "item_2_sub": "供应链攻击防护",
  "item_2_desc": "gVisor userspace kernel — Agents run in sandbox with no root, no TUN, no iptables; sandbox escape cannot reach the host",
  "item_3_title": "Access Management",
  "item_3_sub": "访问管理",
  "item_3_desc": "Policy engine + AgentIdentity CRD — tool-level permission control, sub-agent permissions never exceed parent",
  "item_4_title": "Harmonized Platform Controls",
  "item_4_sub": "统一平台管控",
  "item_4_desc": "Single K8s-native control plane — 13 CRDs manage the full lifecycle of Agents, tunnels, and network policies"
},
"why": {
  "tag": "Our Approach",
  "title": "The Infrastructure Layer for SAIF, Delivered as Open Source",
  "subtitle": "Existing network solutions (Calico, Cilium) were designed for static services. They don't understand AI Agent tool call chains, dynamic delegation, or credential persistence. We built Lattice because every AI Agent deserves a network identity that is auditable, revocable, and permission-scoped.",
  "item_1_title": "Identity-First",
  "item_1_desc": "Every Agent holds a unique WireGuard key pair. Identity is not tied to IP — it's cryptographic, revocable, and auditable",
  "item_2_title": "Full Observability",
  "item_2_desc": "Every MCP tool call automatically records a tool_span (traceId, agentId, duration, status). Delegation chains are fully queryable",
  "item_3_title": "Zero Privilege",
  "item_3_desc": "gVisor sandbox. No root required. One command to start. Zero changes to host machine state"
},
"oss": {
  "tag": "Open Source",
  "title": "Core Capabilities Always Open, Community First",
  "subtitle": "AI infrastructure should not be a black box. WireGuard Mesh, gVisor sandbox, tool call tracing — these core security capabilities are fully open in the Community edition, Apache 2.0 licensed, self-hostable, auditable, and modifiable. PRO adds enterprise extensions, but the open source commitment never changes.",
  "stat_1_value": "Apache 2.0",
  "stat_1_label": "License",
  "stat_2_value": "Go 1.25",
  "stat_2_label": "Language",
  "stat_3_value": "13 CRDs",
  "stat_3_label": "K8s Native",
  "stat_4_value": "Self-hosted",
  "stat_4_label": "Zero lock-in",
  "cta": "View Source on GitHub",
  "cta_sub": "github.com/alatticeio/lattice"
}
```

---

## 实现约束

1. **不改动现有区块**：Hero、Terminal Demo、Two Pillars、Features Grid、Quickstart、Pricing、CTA、Footer 均不修改，只插入新区块
2. **CSS 变量**：所有颜色使用 Tailwind design token（`bg-background`、`text-foreground`、`border-border` 等），不硬编码 hex，明暗自动适配
3. **图标颜色**：用 `bg-blue-500/10 text-blue-500`、`bg-green-500/10 text-green-500` 等 opacity 写法，暗色下同样可见
4. **组件复用**：使用现有 `SectionHeader.vue` 组件渲染 tag + title + subtitle，与其他区块风格一致
5. **i18n 完整**：所有硬编码文字必须进 locale 文件，不留中文或英文字面量在 `.vue` 模板中
6. **SAIF 链接**：`https://safety.google/cybersecurity-advancements/saif/` 用 `target="_blank" rel="noopener noreferrer"`

---

## 文件变更清单

| 文件 | 变更类型 |
|------|---------|
| `frontend/src/pages/index.vue` | 修改：插入 3 个新 section |
| `frontend/src/locales/zh-CN/landing.json` | 修改：新增 `problem`、`why`、`oss` key |
| `frontend/src/locales/en/landing.json` | 修改：新增 `problem`、`why`、`oss` key |
