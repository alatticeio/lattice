# Lattice 官网 & 全站主题设计

> 日期: 2026-05-18
> 性质: 实现规范
> 范围: Home 页重写 + 全站 Dashboard 主题统一 + 通用组件库

---

## 零、设计目标

1. **极客感**：混合风 — 亮色专业底 + 终端/拓扑视觉锚点 + monospace 字体点缀
2. **样式集中**：所有主题变更在 `style.css` 中完成，不做页面级散落样式
3. **组件复用**：新建 4 个 Lattice 风格通用组件，Home 页用，Dashboard 可选接入
4. **最小侵入**：shadcn-vue 组件不改源码，只通过 CSS variables 和 `@layer` 覆盖

---

## 一、色板重新定义

shadcn-vue 当前是 neutral gray 色板。我们把 primary 换成 **indigo** 为主导色：

```css
:root {
  --radius: 0.625rem; /* 保持 shadcn 默认 */

  /* 背景 → 暖白，比纯白柔和 */
  --background: oklch(0.985 0.002 260);
  --foreground: oklch(0.145 0.005 260);

  /* 卡片 → 纯白 */
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.145 0.005 260);

  /* primary → indigo-600 */
  --primary: oklch(0.45 0.18 270);
  --primary-foreground: oklch(0.99 0 0);

  /* secondary → indigo-50 */
  --secondary: oklch(0.95 0.01 265);
  --secondary-foreground: oklch(0.35 0.14 270);

  /* muted → warm gray */
  --muted: oklch(0.965 0.003 260);
  --muted-foreground: oklch(0.5 0.01 260);

  /* accent → cyan */
  --accent: oklch(0.92 0.04 210);
  --accent-foreground: oklch(0.3 0.08 210);

  /* destructive 保持红色 */
  --destructive: oklch(0.58 0.24 27);

  /* border → 稍微偏蓝 */
  --border: oklch(0.91 0.008 262);
  --input: oklch(0.91 0.008 262);
  --ring: oklch(0.5 0.15 270);

  /* chart 色板 → indigo / cyan / violet 系 */
  --chart-1: oklch(0.55 0.18 270);
  --chart-2: oklch(0.6 0.14 210);
  --chart-3: oklch(0.5 0.12 230);
  --chart-4: oklch(0.65 0.2 290);
  --chart-5: oklch(0.7 0.15 180);

  /* sidebar → 稍深 */
  --sidebar: oklch(0.97 0.003 260);
  --sidebar-foreground: oklch(0.145 0.005 260);
  --sidebar-primary: oklch(0.45 0.18 270);
  --sidebar-primary-foreground: oklch(0.99 0 0);
  --sidebar-accent: oklch(0.93 0.01 265);
  --sidebar-accent-foreground: oklch(0.35 0.14 270);
  --sidebar-border: oklch(0.91 0.008 262);
  --sidebar-ring: oklch(0.5 0.15 270);
}
```

dark 模式对称处理，primary 用浅 indigo。

---

## 二、字体系统

```css
@import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600;700&family=Inter:wght@400;500;600;700;800;900&display=swap');

@theme inline {
  --font-sans: 'Inter', ui-sans-serif, system-ui;
  --font-mono: 'JetBrains Mono', ui-monospace;
}
```

全局默认 sans = Inter，代码/终端/数字用 JetBrains Mono。已在 `style.css` 引用的 JetBrains Mono 保留。

---

## 三、@layer lattice 工具类

在 `style.css` 中追加一层，不改 shadcn 基础层：

```css
@layer lattice {
  /* ── 终端组件 ───────────────────────────────── */
  .lattice-terminal {
    @apply font-mono text-sm leading-relaxed;
    background: oklch(0.15 0.01 260);
    color: oklch(0.85 0.02 180);
  }
  .lattice-terminal .prompt      { color: oklch(0.75 0.15 270); }   /* indigo */
  .lattice-terminal .cmd         { color: oklch(0.9 0.02 180); }     /* cyan */
  .lattice-terminal .ok          { color: oklch(0.7 0.16 160); }     /* green */
  .lattice-terminal .warn        { color: oklch(0.75 0.12 80); }     /* amber */
  .lattice-terminal .dim         { color: oklch(0.5 0.01 260); }     /* muted */

  /* ── 渐变文字 ───────────────────────────────── */
  .lattice-gradient-text {
    @apply font-black tracking-tighter;
    background: linear-gradient(135deg,
      oklch(0.45 0.18 270) 0%,
      oklch(0.55 0.16 240) 50%,
      oklch(0.6 0.14 210) 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  /* ── Lattice 卡片 ───────────────────────────── */
  .lattice-card {
    @apply bg-card border border-border rounded-xl;
    box-shadow: 0 1px 2px oklch(0.45 0.18 270 / 0.04);
    transition: box-shadow 0.2s, transform 0.2s;
  }
  .lattice-card:hover {
    box-shadow: 0 4px 12px oklch(0.45 0.18 270 / 0.08);
    transform: translateY(-1px);
  }

  /* ── 拓扑线 ─────────────────────────────────── */
  .lattice-topology-line {
    stroke: oklch(0.45 0.18 270 / 0.3);
    stroke-width: 1;
    fill: none;
  }
  .lattice-topology-node {
    fill: oklch(0.45 0.18 270 / 0.6);
  }

  /* ── 状态标签 ───────────────────────────────── */
  .lattice-badge-stable {
    @apply text-xs font-bold px-2 py-0.5 rounded-full;
    background: oklch(0.7 0.16 160 / 0.1);
    color: oklch(0.45 0.12 160);
  }
  .lattice-badge-roadmap {
    @apply text-xs font-bold px-2 py-0.5 rounded-full;
    background: oklch(0.75 0.12 80 / 0.1);
    color: oklch(0.5 0.1 80);
  }
  .lattice-badge-pro {
    @apply text-xs font-bold px-2 py-0.5 rounded-full;
    background: linear-gradient(135deg, oklch(0.45 0.18 270 / 0.1), oklch(0.6 0.14 210 / 0.1));
    color: oklch(0.45 0.18 270);
  }

  /* ── 数字/统计大字 ──────────────────────────── */
  .lattice-stat-number {
    @apply font-mono font-black text-3xl tracking-tighter;
  }
}
```

---

## 四、新建 4 个通用组件

### 4.1 `LatticeTerminal.vue`

可复用的终端窗口。接受 `lines: string[]`（每行支持 HTML class 标记）和一个 `title`。

```
props:
  - title: string      // 左上角标题（如 "lattice — sandbox"）
  - lines: TerminalLine[]  // { text, cls: 'prompt'|'cmd'|'ok'|'warn'|'dim' }
  - status?: string     // 右上角状态（如 "ONLINE"），不传则不显示
```

视觉：仿 macOS 窗口（红黄绿三个点），暗色背景，monospace 字体，行高舒适。

### 4.2 `TopologyCanvas.vue`

轻量 SVG 力导向拓扑动画（纯 SVG + requestAnimationFrame，不引入 D3.js）。

```
props:
  - nodes: { id, x, y }[]
  - links: { source, target }[]
  - animated?: boolean   // 默认 true
```

视觉：indigo 半透明节点和连线，节点有呼吸动画，放在 Hero 区背景或独立 section。

### 4.3 `StatCard.vue`

Dashboard 数据卡片通用组件，替代 Dashboard 中现有的手写 stat card。

```
props:
  - icon: Component
  - label: string
  - value: number | string
  - color: 'indigo'|'emerald'|'amber'|'violet'|'cyan'
```

使用 `lattice-card` + `lattice-stat-number` 工具类。

### 4.4 `SectionHeader.vue`

页面 section 标题通用组件。

```
props:
  - tag: string      // 小标签（如 "Capabilities"）
  - title: string
  - subtitle?: string
```

---

## 五、Home 页面重写

### 5.1 从当前版本移除的内容

| 移除项 | 原因 |
|--------|------|
| 虚构的 128 nodes / 42ms latency 统计面板 | 不是真实数据，纯装饰用假数字会降低专业感 |
| `#architecture` section + 终端 | 合并到 Two Pillars + Quickstart 中 |
| `ai_sidecar` 卡片（seccomp+eBPF 拦截） | 未实现 |
| `cgroup 限制` / `Sidecar 拦截` / `PID→TUN` 相关 badge | 均未实现，不在 Roadmap 章节里误导 |
| 所有 `lattice-agent-sandbox start` 命令名 | 替换为 `lattice sandbox start` |

### 5.2 页面结构（从上到下）

| Section | 内容 | 视觉锚点 |
|---------|------|---------|
| Navbar | Logo + 导航链接 + 登录/控制台按钮 | 与目前一致，颜色跟主题 |
| Hero | 主标题 + 副标题 + `lattice sandbox start` CTA | 背景：`TopologyCanvas` 动画；文字：`lattice-gradient-text` |
| Terminal Demo | 沙箱启动全过程回放 | `LatticeTerminal` 组件，展示 NATS 注册 → ICE 打洞 → 工具调用追踪 |
| Two Pillars | 左右双栏：网络编排（WireGuard/ICE/LRP） vs Agent 沙箱（gVisor/RBAC/Trace）| `lattice-card` + 图标 |
| Features Grid | 3×2 卡片矩阵 | `lattice-card`，每卡有 stable/roadmap/pro 状态标签 |
| Quickstart Code | 三行命令：`docker run`、`kubectl apply`、`lattice sandbox start` | 代码块 + monospace |
| Comparison Table | 与竞品对比（隐藏 Tailscale/Netbird/ZeroTier） | 精简表格 |
| Pricing | Community vs PRO | 与目前基本一致，颜色跟主题 |
| CTA | 一条命令启动 | 简洁 |
| Footer | 链接 + copyright | 简化为一行 |

### 5.2 Hero 区

```html
<section class="relative overflow-hidden pt-28 pb-24 px-6">
  <!-- 背景拓扑动画 -->
  <TopologyCanvas :nodes="heroNodes" :links="heroLinks" class="absolute inset-0 -z-10" />

  <div class="max-w-3xl mx-auto text-center">
    <span class="lattice-badge-stable inline-flex items-center gap-1.5 mb-6">
      <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
      AI-Native Networking
    </span>

    <h1 class="text-4xl md:text-5xl font-black tracking-tighter leading-[1.1] mb-5">
      <span class="lattice-gradient-text">Zero-Trust Runtime</span>
      <br />
      <span class="text-foreground">for AI Agents</span>
    </h1>

    <p class="text-muted-foreground text-base max-w-lg mx-auto mb-8">
      One binary. Zero privilege. WireGuard-encrypted mesh for every agent.
    </p>

    <div class="flex justify-center gap-3">
      <Button class="bg-primary text-primary-foreground">lattice sandbox start</Button>
      <Button variant="outline">View Console</Button>
    </div>
  </div>
</section>
```

### 5.3 Terminal Demo 区

使用 `LatticeTerminal` 组件，展示当前真实输出：

```
$ lattice sandbox start --name my-agent --token lt-enroll-xxx
  → NATS enrollment...                                              ✓
  → WireGuard keypair generated                                     ✓
  → VPN IP assigned: 10.100.0.5                                    ✓
  Agent "my-agent" online, ICE P2P connected

$ ExecuteTool("list_peers")
  → CheckToolAccess ✓ → execute → 38ms
  → tool_span: traceId=a1b2, status=ok
```

### 5.4 Two Pillars

```
┌──────────────────────────┐  ┌──────────────────────────┐
│  🔗 Network Orchestration│  │  🛡️ AI Agent Sandbox      │
│                          │  │                          │
│  WireGuard Mesh 自动化    │  │  gVisor 零特权网络栈      │
│  ICE P2P + LRP Relay    │  │  Zero-Trust 注册 + JWT    │
│  iptables/eBPF 策略      │  │  Tool RBAC + Trace       │
│  K8s CRD 编排            │  │  Sub-agent 委派           │
│                          │  │                          │
│  [了解更多 →]             │  │  [了解更多 →]             │
└──────────────────────────┘  └──────────────────────────┘
```

### 5.5 Features Grid（3×2）

| 卡片 | 标签 | 内容 |
|------|------|------|
| 零特权沙箱 | stable | `lattice sandbox start`，gVisor pkg/tcpip |
| 工具调用追踪 | stable | tool_spans + traceId + 审计查询 |
| MCP Server | stable | 14 tools，自然语言管理网络 |
| WireGuard Mesh | stable | ICE/STUN P2P + LRP relay |
| eBPF 策略引擎 | pro | TC ingress 内核级硬阻断 |
| Intent Engine | pro | 自然语言 → CRD 变更计划 |

---

## 六、Dashboard 主题应用

Dashboard 页面**不改布局结构**，只通过 `style.css` 全局生效：

1. **所有 shadcn 组件**自动获得 indigo 色板（Button、Card、Badge、Sidebar、Dropdown 等）
2. **Sidebar** — 背景 `--sidebar` 跟上新的 indigo 色板
3. **数字卡片** — 替换为 `StatCard` 组件（逐个页面迁移）
4. **字体** — 全局 Inter + JetBrains Mono 数字

Dashboard 现有页面逐一微调项：

| 页面 | 改动 |
|------|------|
| Dashboard | StatCard 替换手写卡片；颜色跟主题 |
| Nodes | 表头/状态标签色跟主题 |
| Policies | 策略卡片 hover 效果用 `.lattice-card` |
| Sandbox (index) | Agent 列表卡片用 `.lattice-card` |
| AgentDetailDrawer | Traces tab 的 span 状态标签用 `.lattice-badge-*` |
| Tokens | 表格行 hover 跟主题 |
| Topology | 拓扑图节点色跟 `--primary` |
| Members/Users/Settings 等 | 无需改动（CSS variables 自动生效） |

---

## 七、实现顺序

```
Phase 1: style.css 换色 + @layer lattice
  文件: fronted/src/style.css
  验收: 启动 dev server，所有 shadcn 组件自动变为 indigo 色系

Phase 2: 4 个通用组件
  文件: fronted/src/components/lattice/Terminal.vue
        fronted/src/components/lattice/TopologyCanvas.vue
        fronted/src/components/lattice/StatCard.vue
        fronted/src/components/lattice/SectionHeader.vue
  验收: 组件独立可用，props 正确

Phase 3: Home 页重写
  文件: fronted/src/pages/index.vue
  验收: 7 个 section 完整，拓扑动画可动，终端展示正确

Phase 4: Dashboard 页面逐页接入
  文件: dashboard/index.vue 等
  验收: StatCard 替换 + 标签类替换，视觉与 Home 统一

Phase 5: i18n 同步
  文件: zh-CN/landing.json, en/landing.json
  验收: 中英文无缺漏
```
