# Landing Page Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在落地页新增三个区块（The Problem / Why Lattice / Open Source Commitment），引入 Google SAIF 框架叙事，完善「为什么做这个项目」的内容。

**Architecture:** 纯前端改动。新增区块直接插入 `frontend/src/pages/index.vue`，文案统一走 i18n（zh-CN / en），CSS 全部使用现有 design token，自动适配明暗模式。不新建组件，复用现有 `SectionHeader.vue`。

**Tech Stack:** Vue 3.5 · vue-i18n · Tailwind 4 · Lucide Vue

---

## 文件变更清单

| 文件 | 变更 |
|------|------|
| `frontend/src/locales/zh-CN/landing.json` | 新增 `problem` / `why` / `oss` key 组 |
| `frontend/src/locales/en/landing.json` | 新增 `problem` / `why` / `oss` key 组 |
| `frontend/src/pages/index.vue` | 插入三个 `<section>`，无其他改动 |

---

## Task 1：添加 i18n key（zh-CN）

**Files:**
- Modify: `frontend/src/locales/zh-CN/landing.json`

- [ ] **Step 1：在 `footer` key 前插入新 key 组**

  打开 `frontend/src/locales/zh-CN/landing.json`，找到最后的 `"footer"` 对象前面（文件末尾 `"footer": {` 上方），插入以下内容（注意在上一个 key 末尾加逗号）：

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
    },
  ```

- [ ] **Step 2：验证 JSON 合法**

  ```bash
  node -e "JSON.parse(require('fs').readFileSync('frontend/src/locales/zh-CN/landing.json','utf8')); console.log('valid')"
  ```

  预期输出：`valid`（若报错说明 JSON 有语法问题，检查逗号和括号）

- [ ] **Step 3：commit**

  ```bash
  git add frontend/src/locales/zh-CN/landing.json
  git commit -s -m "feat(i18n): add problem/why/oss landing keys for zh-CN"
  ```

---

## Task 2：添加 i18n key（en）

**Files:**
- Modify: `frontend/src/locales/en/landing.json`

- [ ] **Step 1：在 `footer` key 前插入新 key 组**

  打开 `frontend/src/locales/en/landing.json`，在 `"footer": {` 上方插入以下内容：

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
      "item_1_desc": "WireGuard Encrypted Mesh — every Agent holds its own key pair, traffic is end-to-end encrypted, control and data planes are separated",
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
    },
  ```

- [ ] **Step 2：验证 JSON 合法**

  ```bash
  node -e "JSON.parse(require('fs').readFileSync('frontend/src/locales/en/landing.json','utf8')); console.log('valid')"
  ```

  预期输出：`valid`

- [ ] **Step 3：commit**

  ```bash
  git add frontend/src/locales/en/landing.json
  git commit -s -m "feat(i18n): add problem/why/oss landing keys for en"
  ```

---

## Task 3：添加 The Problem 区块

**背景：** 该区块插入在 Terminal Demo 之后（`</section>` 约第 252 行）、Two Pillars 之前（`<!-- ── Two Pillars` 约第 254 行）。

**Files:**
- Modify: `frontend/src/pages/index.vue:252`

- [ ] **Step 1：在 Terminal Demo `</section>` 和 Two Pillars 注释之间插入**

  找到以下标记（约 252-254 行）：
  ```html
      </section>

      <!-- ── Two Pillars ────────────────────────────────────────────── -->
  ```

  在两者之间插入：
  ```html
      <!-- ── The Problem ───────────────────────────────────────────── -->
      <section id="problem" class="py-20 px-6 bg-muted/50 border-y border-border">
        <div class="max-w-4xl mx-auto">
          <SectionHeader
            :tag="t('landing.problem.tag')"
            :title="t('landing.problem.title')"
            :subtitle="t('landing.problem.subtitle')"
          />

          <div class="space-y-3 mb-8">
            <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground text-center mb-5">
              {{ t('landing.problem.saif_label') }}
            </p>
            <div v-for="item in saifItems" :key="item.key"
              class="flex items-start gap-4 p-4 rounded-xl border border-border bg-background">
              <div :class="item.iconBg" class="size-10 rounded-lg flex items-center justify-center shrink-0 text-xl">
                {{ item.icon }}
              </div>
              <div>
                <p class="text-sm font-bold text-foreground">
                  {{ t('landing.problem.' + item.key + '_title') }}
                  <span class="text-xs font-normal text-muted-foreground ml-1.5">
                    {{ t('landing.problem.' + item.key + '_sub') }}
                  </span>
                </p>
                <p class="text-xs text-muted-foreground mt-1 leading-relaxed">
                  {{ t('landing.problem.' + item.key + '_desc') }}
                </p>
              </div>
            </div>
          </div>

          <p class="text-center text-xs text-muted-foreground">
            {{ t('landing.problem.ref_prefix') }}<a
              href="https://safety.google/cybersecurity-advancements/saif/"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary hover:underline underline-offset-4"
            >{{ t('landing.problem.ref_link') }}</a>{{ t('landing.problem.ref_suffix') }}
          </p>
        </div>
      </section>

  ```

- [ ] **Step 2：在 `<script setup>` 中添加 `saifItems` 数据**

  在 `index.vue` 的 `<script setup>` 里，找到 `const terminalLines` 的定义之后，追加：

  ```typescript
  const saifItems = [
    { key: 'item_1', icon: '🔒', iconBg: 'bg-blue-500/10 text-blue-600 dark:text-blue-400' },
    { key: 'item_2', icon: '🛡', iconBg: 'bg-green-500/10 text-green-600 dark:text-green-400' },
    { key: 'item_3', icon: '🎯', iconBg: 'bg-violet-500/10 text-violet-600 dark:text-violet-400' },
    { key: 'item_4', icon: '⚙', iconBg: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  ]
  ```

- [ ] **Step 3：启动开发服务器验证**

  ```bash
  cd frontend && pnpm dev
  ```

  在浏览器打开 `http://localhost:5173`，滚动到 Terminal Demo 下方，确认：
  - The Problem 区块出现，背景为 `bg-muted/50`
  - 4 张卡片均显示，图标背景色正常
  - 切换深色模式，图标色仍可见
  - 切换中英文，文字均正确翻译

- [ ] **Step 4：commit**

  ```bash
  git add frontend/src/pages/index.vue
  git commit -s -m "feat(landing): add The Problem section with SAIF mapping"
  ```

---

## Task 4：添加 Why Lattice 区块

**背景：** 该区块插入在 The Problem 区块之后、Two Pillars 之前（紧跟 Task 3 插入的内容）。

**Files:**
- Modify: `frontend/src/pages/index.vue`

- [ ] **Step 1：在 The Problem `</section>` 和 Two Pillars 注释之间插入**

  找到以下标记（Task 3 完成后存在）：
  ```html
      </section>

      <!-- ── Two Pillars ────────────────────────────────────────────── -->
  ```

  在两者之间插入：
  ```html
      <!-- ── Why Lattice ───────────────────────────────────────────── -->
      <section id="why" class="py-20 px-6">
        <div class="max-w-4xl mx-auto">
          <SectionHeader
            :tag="t('landing.why.tag')"
            :title="t('landing.why.title')"
            :subtitle="t('landing.why.subtitle')"
          />

          <div class="grid grid-cols-1 md:grid-cols-3 gap-5">
            <div v-for="item in whyItems" :key="item.key"
              class="p-6 rounded-xl border border-border bg-muted/50">
              <div class="text-2xl mb-4">{{ item.icon }}</div>
              <p class="text-sm font-bold text-foreground mb-2">{{ t('landing.why.' + item.key + '_title') }}</p>
              <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.why.' + item.key + '_desc') }}</p>
            </div>
          </div>
        </div>
      </section>

  ```

- [ ] **Step 2：在 `<script setup>` 中添加 `whyItems` 数据**

  在 `saifItems` 定义之后追加：

  ```typescript
  const whyItems = [
    { key: 'item_1', icon: '🔑' },
    { key: 'item_2', icon: '📋' },
    { key: 'item_3', icon: '🚫' },
  ]
  ```

- [ ] **Step 3：浏览器验证**

  在 `http://localhost:5173` 确认：
  - Why Lattice 区块出现在 The Problem 之后，背景为纯色（与 The Problem 的 muted 交替）
  - 三列卡片在桌面端正确显示为三列，移动端堆叠为单列
  - 切换中英文正确翻译

- [ ] **Step 4：commit**

  ```bash
  git add frontend/src/pages/index.vue
  git commit -s -m "feat(landing): add Why Lattice section with three value cards"
  ```

---

## Task 5：添加 Open Source Commitment 区块

**背景：** 该区块插入在 Pricing `</section>` 之后（约第 454 行）、CTA 注释之前（约第 455 行）。

**Files:**
- Modify: `frontend/src/pages/index.vue:454`

- [ ] **Step 1：在 Pricing `</section>` 和 CTA 注释之间插入**

  找到以下标记（约 454-456 行）：
  ```html
      </section>

      <!-- ── CTA ────────────────────────────────────────────────────── -->
  ```

  在两者之间插入：
  ```html
      <!-- ── Open Source ───────────────────────────────────────────── -->
      <section id="open-source" class="py-20 px-6 bg-muted/50 border-y border-border">
        <div class="max-w-4xl mx-auto">
          <SectionHeader
            :tag="t('landing.oss.tag')"
            :title="t('landing.oss.title')"
            :subtitle="t('landing.oss.subtitle')"
          />

          <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
            <div v-for="item in ossStats" :key="item.value"
              class="text-center p-5 rounded-xl border border-border bg-background">
              <p class="text-lg font-black tracking-tight text-foreground">{{ t('landing.oss.' + item.valueKey) }}</p>
              <p class="text-xs text-muted-foreground mt-1">{{ t('landing.oss.' + item.labelKey) }}</p>
            </div>
          </div>

          <div class="flex flex-col sm:flex-row items-center justify-center gap-3">
            <Button
              variant="outline"
              size="lg"
              class="gap-2 border-border"
              as="a"
              href="https://github.com/alatticeio/lattice"
              target="_blank"
              rel="noopener noreferrer"
            >
              <svg class="size-4" viewBox="0 0 98 96" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
                <path fill-rule="evenodd" clip-rule="evenodd" d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z"/>
              </svg>
              {{ t('landing.oss.cta') }}
            </Button>
            <span class="text-xs text-muted-foreground font-mono">{{ t('landing.oss.cta_sub') }}</span>
          </div>
        </div>
      </section>

  ```

- [ ] **Step 2：在 `<script setup>` 中添加 `ossStats` 数据**

  在 `whyItems` 定义之后追加：

  ```typescript
  const ossStats = [
    { valueKey: 'stat_1_value', labelKey: 'stat_1_label' },
    { valueKey: 'stat_2_value', labelKey: 'stat_2_label' },
    { valueKey: 'stat_3_value', labelKey: 'stat_3_label' },
    { valueKey: 'stat_4_value', labelKey: 'stat_4_label' },
  ]
  ```

- [ ] **Step 3：浏览器验证**

  在 `http://localhost:5173` 确认：
  - Open Source 区块出现在 Pricing 之后、CTA 之前
  - 四格统计在桌面端为四列，移动端为两列
  - GitHub 按钮点击可跳转（`target="_blank"`）
  - 明暗模式下均正常显示
  - 中英文切换正确

- [ ] **Step 4：commit**

  ```bash
  git add frontend/src/pages/index.vue
  git commit -s -m "feat(landing): add Open Source Commitment section"
  ```

---

## Task 6：最终整体验证

**Files:** 无新修改，仅验证。

- [ ] **Step 1：检查所有新增 i18n key 是否被正确引用**

  ```bash
  grep -n "landing\.problem\.\|landing\.why\.\|landing\.oss\." frontend/src/pages/index.vue
  ```

  预期：输出若干行，每个 key 均有对应引用，无拼写错误。

- [ ] **Step 2：检查无硬编码中英文字面量残留**

  ```bash
  # 检查新增区块中是否有未走 i18n 的中文
  grep -n "[\u4e00-\u9fff]" frontend/src/pages/index.vue
  ```

  预期：仅有注释（`<!-- ── ... ──>`）中的中文，无模板内文字字面量。

- [ ] **Step 3：lint 检查**

  ```bash
  cd frontend && pnpm lint 2>&1 | head -30
  ```

  预期：无 error，仅可能有 warning（如已有的 warning 不算新增问题）。

- [ ] **Step 4：完整浏览验证流程**

  启动 `pnpm dev`，按以下清单逐项检查：

  | 检查项 | 预期 |
  |--------|------|
  | 页面顺序 | Terminal → The Problem → Why Lattice → Two Pillars → Features → Quickstart → Pricing → Open Source → CTA |
  | 交替背景 | The Problem = `bg-muted/50`，Why Lattice = 纯色，Open Source = `bg-muted/50`，与相邻区块形成节奏 |
  | SAIF 卡片图标 | 4 个图标背景色：蓝/绿/紫/橙，深色模式下可见 |
  | Why Lattice 三列 | 桌面端三列，移动端（< 768px）单列堆叠 |
  | Open Source 统计格 | 桌面端四列，移动端两列 |
  | SAIF 论文链接 | 点击跳转 `https://safety.google/cybersecurity-advancements/saif/`，新标签打开 |
  | GitHub 按钮 | 点击跳转 `https://github.com/alatticeio/lattice`，新标签打开 |
  | 语言切换 | 所有新增区块中英文均正确显示 |
  | 明暗切换 | 所有新增区块在 light/dark 模式下均正常渲染 |

- [ ] **Step 5：commit**

  ```bash
  git add -p  # 确认无意外改动
  git commit -s -m "feat(landing): complete SAIF narrative redesign — problem/why/oss sections"
  ```
