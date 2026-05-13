# Home Page Redesign & Agent Sandbox Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite Home landing page with AI Agent sandbox as the new product narrative, add Agent Sandbox management pages to the dashboard.

**Architecture:** Vue 3 SFC pages + Pinia store + i18n (zh-CN/en). Home page is a single SFC (`pages/index.vue`) using `blank` layout. New sandbox pages use `default` layout with sidebar navigation. API layer follows existing `request.get/post/delete` pattern.

**Tech Stack:** Vue 3.5, Pinia, vue-i18n, lucide-vue-next, Tailwind 4, Vitest

---

## File Structure

```
Create:
  fronted/src/api/sandbox.ts              # Sandbox API client
  fronted/src/stores/useSandboxStore.ts   # Sandbox state management
  fronted/src/pages/sandbox/index.vue     # Sandbox list page
  fronted/src/pages/sandbox/tokens.vue    # Enrollment tokens page
  fronted/src/pages/sandbox/audit.vue     # Traffic audit page

Modify:
  fronted/src/pages/index.vue            # Home page full rewrite
  fronted/src/locales/zh-CN/landing.json  # Landing i18n (CN)
  fronted/src/locales/en/landing.json     # Landing i18n (EN)
  fronted/src/locales/zh-CN/common.json   # Common i18n (CN) + sandbox nav
  fronted/src/locales/en/common.json      # Common i18n (EN) + sandbox nav
  fronted/src/components/app-sidebar/AppSidebar.vue  # Add sandbox nav group
  fronted/src/pages/dashboard/index.vue   # Add 5th stat card
```

---

### Task 1: Landing Page i18n Keys

**Files:**
- Modify: `fronted/src/locales/zh-CN/landing.json`
- Modify: `fronted/src/locales/en/landing.json`

Replace the entire landing.json files to match the new page content. The existing keys for `nav` and `footer` sections stay largely the same; Hero, Features, Architecture, Pricing, CTA sections are rewritten.

- [ ] **Step 1: Write CN landing.json**

```json
{
  "nav": {
    "features": "功能",
    "architecture": "架构",
    "pricing": "定价",
    "quickstart": "快速开始",
    "login": "登录",
    "console": "控制台",
    "logout": "退出",
    "docs": "文档",
    "github": "GitHub",
    "community": "社区"
  },
  "hero": {
    "badge": "AI-Native 网络",
    "title": "AI Agent 的零信任运行环境",
    "subtitle": "Lattice 为每个 AI Agent 提供零特权沙箱隔离，通过 WireGuard 加密 Mesh 安全接入你的基础设施——从启动到组网，一条命令。",
    "cta_primary": "lattice-agent-sandbox start",
    "cta_secondary": "查看控制台"
  },
  "terminal": {
    "title": "lattice — control-plane",
    "status": "AI SANDBOX ONLINE",
    "line1": "$ lattice-agent-sandbox start --name my-agent",
    "line2": "  → Creating gVisor sandbox...      ✓",
    "line3": "  → Attaching WireGuard tunnel...   ✓",
    "line4": "  → Registering with control plane.. ✓",
    "line5": "  Agent \"my-agent\" is online at 10.100.0.5",
    "line6": "# 合法外联 (白名单命中)",
    "line7": "$ curl https://api.internal",
    "line8": "  → PolicyCache.Allow() → ✓ → wireguard-go encrypt",
    "line9": "  → P2P/Relay → 远端解密 → 200 OK",
    "line10": "# 违规外联 (未命中)",
    "line11": "$ curl https://evil.com",
    "line12": "  → PolicyCache.Allow() → ✗ → EACCES",
    "line13": "  → auditCh → /api/v1/audit/batch → 控制面告警"
  },
  "features": {
    "label": "核心能力",
    "title": "AI Agent 安全运行时 + 网络基础层",
    "subtitle": "Lattice 提供双层能力：上层为 AI Agent 提供零特权沙箱运行环境，下层为所有工作负载提供 WireGuard 加密网络。",
    "tag_stable": "稳定版",
    "tag_roadmap": "路线图",
    "ai_sandbox": {
      "title": "零特权沙箱隔离",
      "desc": "gVisor 用户态内核运行 Agent，无需 root/CAP_NET_ADMIN。每个 Agent 拥有独立 Go netstack，PID→TUN 绑定，外联全管控。"
    },
    "ai_sidecar": {
      "title": "Sidecar 外联拦截",
      "desc": "seccomp notify + eBPF fast path 双重拦截。Agent 每次 socket 操作向 PolicyCache 查表，白名单命中直通，未命中→EACCES+审计。"
    },
    "ai_intent": {
      "title": "意图驱动网络管理",
      "desc": "用自然语言描述网络需求——\"allow frontend to access api on 8080\"——AI 自动生成变更计划，Diff 预览后通过 RBAC 审批生效。"
    },
    "net_wg": {
      "title": "WireGuard 加密 Mesh",
      "desc": "NAT 穿透 P2P 直连 + LRP Relay 中继。WireGuard 端到端加密，ICE/STUN 自动打洞，跨集群桥接零配置。"
    },
    "net_ebpf": {
      "title": "eBPF 高性能策略",
      "desc": "内核态 LPM Trie + Port Hash 策略引擎，TC ingress/egress 挂载，百万规则级匹配。基础设施层东西向流量线速转发。"
    },
    "net_audit": {
      "title": "全局流量审计",
      "desc": "gVisor Go 层 + eBPF ring buffer 双路径汇聚，批量上报控制面，异常模式实时告警。每一次 allow/drop 都可追溯。"
    }
  },
  "architecture": {
    "label": "架构",
    "title": "从沙箱启动到加密通信",
    "subtitle": "零特权架构：Agent 进程在 gVisor 用户态内核中运行，WireGuard 加密在用户态完成，全程无需 root。",
    "step_1_title": "创建沙箱",
    "step_1_desc": "lattice-agent-sandbox start 生成 WireGuard 密钥对，创建 gVisor OCI 容器，Go netstack 接管网络栈。",
    "step_2_title": "注册 + 策略注入",
    "step_2_desc": "Agent 注册到控制面，创建 LatticePeer + AgentIdentity(Sandbox=gvisor)，注入 PolicyChecker 和 AuditWriter。",
    "step_3_title": "Agent 运行与管控",
    "step_3_desc": "每次 socket 操作经 Sentry netstack 拦截 → PolicyCache 查表 → 放行/WG 加密/发送 或 拒绝/EACCES/审计。",
    "terminal_comment": "# 合法流量 (allow verdict path)",
    "terminal_line1": "agent-1 socket → Sentry netstack → Allow",
    "terminal_line2": "  → wireguard-go ChaCha20 encrypt",
    "terminal_line3": "  → UDP :51820 → FilteringUDPMux",
    "terminal_line4": "  → ICE P2P / LRP Relay → 远端 peer",
    "terminal_line5": "  → decrypt → target ✓",
    "terminal_comment2": "# 违规流量 (drop verdict path)",
    "terminal_line6": "agent-1 socket → Sentry netstack → Deny",
    "terminal_line7": "  → EACCES 返回调用方",
    "terminal_line8": "  → ns.auditCh → AuditBatcher",
    "terminal_line9": "  → POST /api/v1/audit/batch → 告警"
  },
  "pricing": {
    "label": "定价",
    "title": "选择适合你的版本",
    "subtitle": "社区版永远免费，Pro 版解锁企业级安全能力。",
    "community_name": "社区版",
    "community_price": "免费",
    "community_desc": "适合个人开发者和 AI Agent 探索者",
    "community_cta": "在 GitHub 上查看",
    "community_feat_1": "WireGuard 加密 Mesh：无限节点",
    "community_feat_2": "CRD 原生 K8s 控制器",
    "community_feat_3": "NATS 信令 + ICE/STUN P2P 打洞",
    "community_feat_4": "LRP Relay 中继",
    "community_feat_5": "Agent 沙箱 (cgroup)：PID 绑定 + 资源限制",
    "community_feat_6": "Sidecar 外联拦截：seccomp + eBPF fast path",
    "community_feat_7": "全局拓扑图可视化",
    "pro_name": "Pro",
    "pro_price": "联系我们",
    "pro_period": "",
    "pro_desc": "适合需要生产级安全的企业和团队",
    "pro_cta": "升级到 Pro",
    "pro_disclaimer": "30 天免费试用，无需信用卡",
    "pro_badge": "推荐",
    "pro_feat_all": "所有社区版功能",
    "pro_feat_1": "gVisor 零特权沙箱隔离：Go netstack 替代 TUN",
    "pro_feat_2": "eBPF TC 策略引擎 (LPM/Port) + 流量镜像审计",
    "pro_feat_3": "意图引擎：自然语言网络变更计划",
    "pro_feat_4": "合规审计扫描 + 异常告警",
    "pro_feat_5": "Kubernetes 集群互联",
    "pro_feat_6": "SSO/OIDC + RBAC + 审批工作流",
    "pro_feat_7": "审计日志 + Webhook 通知",
    "pro_feat_8": "Firecracker MicroVM 沙箱 (远期增强)",
    "pro_feat_locked_1": "gVisor 零特权沙箱隔离",
    "pro_feat_locked_2": "eBPF 流量审计 + 异常告警",
    "pro_feat_locked_3": "意图引擎 (自然语言网络管理)",
    "enterprise_text": "需要大规模部署？",
    "enterprise_link": "联系企业销售 →"
  },
  "cta": {
    "title": "为你的 AI Agent 构建安全运行环境",
    "subtitle": "一条命令启动沙箱，零特权隔离，端到端加密。开始为你的 AI Agent 构建零信任网络。",
    "button_primary": "lattice-agent-sandbox start",
    "button_secondary": "查看控制台",
    "badge_1": "零特权",
    "badge_2": "gVisor 隔离",
    "badge_3": "WireGuard 加密",
    "badge_4": "cgroup 限制",
    "badge_5": "Sidecar 拦截",
    "badge_6": "流量审计"
  },
  "footer": {
    "copyright": "© 2026 Lattice. All rights reserved."
  },
  "stats": {
    "active_nodes": "活跃沙箱",
    "all_healthy": "全部健康",
    "avg_latency": "平均延迟",
    "sync": "最后同步",
    "data_plane": "数据面"
  },
  "advantages": {
    "item_1": "零特权 — 普通用户运行",
    "item_2": "gVisor 用户态内核隔离",
    "item_3": "WireGuard 端到端加密",
    "item_4": "自然语言网络管理",
    "item_5": "eBPF 高性能策略引擎",
    "item_6": "全流量可审计可追溯"
  }
}
```

- [ ] **Step 2: Write EN landing.json**

```json
{
  "nav": {
    "features": "Features",
    "architecture": "Architecture",
    "pricing": "Pricing",
    "quickstart": "Quickstart",
    "login": "Login",
    "console": "Console",
    "logout": "Logout",
    "docs": "Docs",
    "github": "GitHub",
    "community": "Community"
  },
  "hero": {
    "badge": "AI-Native Networking",
    "title": "Zero-Trust Runtime for AI Agents",
    "subtitle": "Lattice gives every AI agent a zero-privilege sandbox and WireGuard-encrypted network access to your infrastructure — from launch to mesh, a single command.",
    "cta_primary": "lattice-agent-sandbox start",
    "cta_secondary": "View Console"
  },
  "terminal": {
    "title": "lattice — control-plane",
    "status": "AI SANDBOX ONLINE",
    "line1": "$ lattice-agent-sandbox start --name my-agent",
    "line2": "  → Creating gVisor sandbox...      ✓",
    "line3": "  → Attaching WireGuard tunnel...   ✓",
    "line4": "  → Registering with control plane.. ✓",
    "line5": "  Agent \"my-agent\" is online at 10.100.0.5",
    "line6": "# Allowed egress (allowlist hit)",
    "line7": "$ curl https://api.internal",
    "line8": "  → PolicyCache.Allow() → ✓ → wireguard-go encrypt",
    "line9": "  → P2P/Relay → remote decrypt → 200 OK",
    "line10": "# Blocked egress (no match)",
    "line11": "$ curl https://evil.com",
    "line12": "  → PolicyCache.Allow() → ✗ → EACCES",
    "line13": "  → auditCh → /api/v1/audit/batch → alert"
  },
  "features": {
    "label": "Capabilities",
    "title": "AI Agent Runtime + Network Foundation",
    "subtitle": "Lattice delivers two layers: the upper layer provides zero-privilege sandboxing for AI agents, and the lower layer provides WireGuard-encrypted networking for all workloads.",
    "tag_stable": "Stable",
    "tag_roadmap": "Roadmap",
    "ai_sandbox": {
      "title": "Zero-Privilege Sandbox",
      "desc": "Run AI agents in gVisor user-space kernel — no root, no CAP_NET_ADMIN. Each agent gets its own Go netstack, PID-to-TUN binding, and full egress control."
    },
    "ai_sidecar": {
      "title": "Sidecar Egress Interception",
      "desc": "Dual-layer enforcement with seccomp notify + eBPF fast path. Every socket operation checks PolicyCache — allowlist hit goes through, miss returns EACCES with audit."
    },
    "ai_intent": {
      "title": "Intent-Driven Network Management",
      "desc": "Describe network needs in natural language — \"allow frontend to access api on 8080\" — AI generates a change plan with diff preview and RBAC approval."
    },
    "net_wg": {
      "title": "WireGuard Encrypted Mesh",
      "desc": "NAT-traversing P2P direct connections + LRP relay fallback. WireGuard end-to-end encryption, ICE/STUN auto hole-punching, zero-config cross-cluster bridging."
    },
    "net_ebpf": {
      "title": "eBPF High-Performance Policy",
      "desc": "Kernel-level LPM Trie + Port Hash policy engine, TC ingress/egress hooks, million-rule matching. Line-rate east-west traffic handling for infrastructure nodes."
    },
    "net_audit": {
      "title": "Global Traffic Audit",
      "desc": "Dual-path aggregation: gVisor Go layer + eBPF ring buffer. Batch-report to the control plane, real-time anomaly alerting. Every allow/drop event is traceable."
    }
  },
  "architecture": {
    "label": "Architecture",
    "title": "From Sandbox Launch to Encrypted Communication",
    "subtitle": "Zero-privilege architecture: agent processes run inside gVisor user-space kernel, WireGuard encryption happens entirely in user space — no root required.",
    "step_1_title": "Create Sandbox",
    "step_1_desc": "lattice-agent-sandbox start generates WireGuard keypair, creates gVisor OCI container, Go netstack takes over the network stack.",
    "step_2_title": "Register + Policy Injection",
    "step_2_desc": "Agent registers with the control plane, creating LatticePeer + AgentIdentity(Sandbox=gvisor), injecting PolicyChecker and AuditWriter hooks.",
    "step_3_title": "Agent Runtime & Enforcement",
    "step_3_desc": "Every socket op intercepted by Sentry netstack → PolicyCache lookup → forward/WG encrypt/send or deny/EACCES/audit.",
    "terminal_comment": "# Allowed traffic (allow verdict path)",
    "terminal_line1": "agent-1 socket → Sentry netstack → Allow",
    "terminal_line2": "  → wireguard-go ChaCha20 encrypt",
    "terminal_line3": "  → UDP :51820 → FilteringUDPMux",
    "terminal_line4": "  → ICE P2P / LRP Relay → remote peer",
    "terminal_line5": "  → decrypt → target ✓",
    "terminal_comment2": "# Blocked traffic (drop verdict path)",
    "terminal_line6": "agent-1 socket → Sentry netstack → Deny",
    "terminal_line7": "  → EACCES returned to caller",
    "terminal_line8": "  → ns.auditCh → AuditBatcher",
    "terminal_line9": "  → POST /api/v1/audit/batch → alert"
  },
  "pricing": {
    "label": "Pricing",
    "title": "Choose Your Edition",
    "subtitle": "Community edition is free forever. Pro unlocks enterprise security capabilities.",
    "community_name": "Community",
    "community_price": "Free",
    "community_desc": "For individual developers and AI agent explorers",
    "community_cta": "View on GitHub",
    "community_feat_1": "WireGuard Encrypted Mesh: unlimited nodes",
    "community_feat_2": "CRD-Native K8s Controller",
    "community_feat_3": "NATS Signaling + ICE/STUN P2P",
    "community_feat_4": "LRP Relay",
    "community_feat_5": "Agent Sandbox (cgroup): PID binding + resource limits",
    "community_feat_6": "Sidecar Egress Interception: seccomp + eBPF fast path",
    "community_feat_7": "Global Topology Visualization",
    "pro_name": "Pro",
    "pro_price": "Contact Us",
    "pro_period": "",
    "pro_desc": "For teams and enterprises needing production-grade security",
    "pro_cta": "Upgrade to Pro",
    "pro_disclaimer": "30-day free trial, no credit card required",
    "pro_badge": "Recommended",
    "pro_feat_all": "Everything in Community",
    "pro_feat_1": "gVisor Zero-Privilege Sandbox: Go netstack replaces TUN",
    "pro_feat_2": "eBPF TC Policy Engine (LPM/Port) + Traffic Mirror Audit",
    "pro_feat_3": "Intent Engine: natural language network change plans",
    "pro_feat_4": "Compliance Audit Scanning + Anomaly Alerts",
    "pro_feat_5": "Kubernetes Cluster Peering",
    "pro_feat_6": "SSO/OIDC + RBAC + Approval Workflows",
    "pro_feat_7": "Audit Logs + Webhook Notifications",
    "pro_feat_8": "Firecracker MicroVM Sandbox (future)",
    "pro_feat_locked_1": "gVisor Zero-Privilege Sandbox Isolation",
    "pro_feat_locked_2": "eBPF Traffic Audit + Anomaly Alerts",
    "pro_feat_locked_3": "Intent Engine (natural language network management)",
    "enterprise_text": "Need large-scale deployment?",
    "enterprise_link": "Contact Enterprise Sales →"
  },
  "cta": {
    "title": "Build a Secure Runtime for Your AI Agents",
    "subtitle": "One command to launch a sandbox. Zero privilege. End-to-end encryption. Start building the zero-trust network for your AI agents.",
    "button_primary": "lattice-agent-sandbox start",
    "button_secondary": "View Console",
    "badge_1": "Zero Privilege",
    "badge_2": "gVisor Isolation",
    "badge_3": "WireGuard Encryption",
    "badge_4": "cgroup Limits",
    "badge_5": "Sidecar Interception",
    "badge_6": "Traffic Audit"
  },
  "footer": {
    "copyright": "© 2026 Lattice. All rights reserved."
  },
  "stats": {
    "active_nodes": "Active Sandboxes",
    "all_healthy": "All Healthy",
    "avg_latency": "Avg Latency",
    "sync": "Last Sync",
    "data_plane": "Data Plane"
  },
  "advantages": {
    "item_1": "Zero-Privilege — runs as normal user",
    "item_2": "gVisor user-space kernel isolation",
    "item_3": "WireGuard end-to-end encryption",
    "item_4": "Natural language network management",
    "item_5": "eBPF high-performance policy engine",
    "item_6": "Full traffic audit and traceability"
  }
}
```

- [ ] **Step 3: Commit**

```bash
git add fronted/src/locales/zh-CN/landing.json fronted/src/locales/en/landing.json
git commit -s -m "feat(frontend): rewrite landing i18n keys for AI agent sandbox narrative"
```

---

### Task 2: Common i18n Keys for Sandbox Navigation

**Files:**
- Modify: `fronted/src/locales/zh-CN/common.json`
- Modify: `fronted/src/locales/en/common.json`

Add sandbox-related keys to the existing `nav.group` and `nav` sections, and add a new `sandbox` top-level section.

- [ ] **Step 1: Add keys to zh-CN/common.json**

Add within the `nav` object (keep all existing keys, only show additions):

```json
"nav": {
  "group": {
    "sandbox": "Agent 沙箱"
  },
  "sandboxList": "沙箱列表",
  "sandboxTokens": "接入令牌",
  "sandboxAudit": "流量审计"
}
```

Add a new top-level `sandbox` section at the end of the file:

```json
"sandbox": {
  "list": {
    "title": "Agent 沙箱",
    "desc": "管理所有 AI Agent 沙箱实例",
    "colName": "名称",
    "colStatus": "状态",
    "colMode": "隔离模式",
    "colIP": "VPN IP",
    "colSandboxId": "Sandbox ID",
    "colTrafficRx": "下行流量",
    "colTrafficTx": "上行流量",
    "colCreated": "创建时间",
    "colActions": "操作",
    "revoke": "吊销",
    "confirmRevoke": "确认吊销沙箱",
    "confirmRevokeDesc": "此操作将立即终止 Agent 进程并移除其网络访问权限，不可逆。",
    "online": "在线",
    "offline": "离线",
    "modeGvisor": "gVisor",
    "modeCgroup": "cgroup",
    "empty": "暂无沙箱实例",
    "error": "加载沙箱列表失败"
  },
  "tokens": {
    "title": "接入令牌",
    "desc": "创建和管理 Agent 注册令牌",
    "createTitle": "创建令牌",
    "createDesc": "生成一次性 enrollment token，Agent 凭此注册到控制面。",
    "ttl": "有效期",
    "ttl1h": "1 小时",
    "ttl6h": "6 小时",
    "ttl24h": "24 小时",
    "allowedTools": "允许的工具",
    "generate": "生成令牌",
    "generatedToken": "令牌已生成",
    "tokenWarning": "此令牌仅在此时可见一次，请立即复制保存。关闭后将无法再次查看。",
    "colToken": "令牌",
    "colCreated": "创建时间",
    "colExpires": "过期时间",
    "colStatus": "状态",
    "colActions": "操作",
    "statusActive": "有效",
    "statusExpired": "已过期",
    "statusRevoked": "已撤销",
    "revoke": "撤销",
    "empty": "暂无令牌",
    "error": "加载令牌列表失败"
  },
  "audit": {
    "title": "流量审计",
    "desc": "Agent 流量事件审计与追溯",
    "searchPlaceholder": "搜索沙箱名或目标 IP...",
    "filterVerdict": "判决",
    "filterAll": "全部",
    "filterAllow": "放行",
    "filterDrop": "拒绝",
    "colTime": "时间",
    "colSandbox": "沙箱",
    "colSrcIP": "源 IP",
    "colDst": "目标 IP:Port",
    "colProtocol": "协议",
    "colVerdict": "判决",
    "allow": "放行",
    "drop": "拒绝",
    "empty": "暂无流量审计事件",
    "noMatch": "未找到匹配的事件",
    "error": "加载审计事件失败"
  }
}
```

- [ ] **Step 2: Add keys to en/common.json**

Same structure, English values:

```json
"nav": {
  "group": {
    "sandbox": "Agent Sandbox"
  },
  "sandboxList": "Sandbox List",
  "sandboxTokens": "Access Tokens",
  "sandboxAudit": "Traffic Audit"
}
```

```json
"sandbox": {
  "list": {
    "title": "Agent Sandboxes",
    "desc": "Manage all AI Agent sandbox instances",
    "colName": "Name",
    "colStatus": "Status",
    "colMode": "Isolation Mode",
    "colIP": "VPN IP",
    "colSandboxId": "Sandbox ID",
    "colTrafficRx": "Rx Traffic",
    "colTrafficTx": "Tx Traffic",
    "colCreated": "Created",
    "colActions": "Actions",
    "revoke": "Revoke",
    "confirmRevoke": "Confirm Revoke Sandbox",
    "confirmRevokeDesc": "This will immediately terminate the agent process and revoke its network access. This action is irreversible.",
    "online": "Online",
    "offline": "Offline",
    "modeGvisor": "gVisor",
    "modeCgroup": "cgroup",
    "empty": "No sandbox instances",
    "error": "Failed to load sandbox list"
  },
  "tokens": {
    "title": "Access Tokens",
    "desc": "Create and manage agent enrollment tokens",
    "createTitle": "Create Token",
    "createDesc": "Generate a one-time enrollment token for agents to register with the control plane.",
    "ttl": "TTL",
    "ttl1h": "1 Hour",
    "ttl6h": "6 Hours",
    "ttl24h": "24 Hours",
    "allowedTools": "Allowed Tools",
    "generate": "Generate Token",
    "generatedToken": "Token Generated",
    "tokenWarning": "This token is only visible once. Copy it now — you won't be able to see it again.",
    "colToken": "Token",
    "colCreated": "Created",
    "colExpires": "Expires",
    "colStatus": "Status",
    "colActions": "Actions",
    "statusActive": "Active",
    "statusExpired": "Expired",
    "statusRevoked": "Revoked",
    "revoke": "Revoke",
    "empty": "No tokens",
    "error": "Failed to load tokens"
  },
  "audit": {
    "title": "Traffic Audit",
    "desc": "Agent traffic event audit and traceability",
    "searchPlaceholder": "Search by sandbox name or destination IP...",
    "filterVerdict": "Verdict",
    "filterAll": "All",
    "filterAllow": "Allow",
    "filterDrop": "Drop",
    "colTime": "Time",
    "colSandbox": "Sandbox",
    "colSrcIP": "Source IP",
    "colDst": "Dest IP:Port",
    "colProtocol": "Protocol",
    "colVerdict": "Verdict",
    "allow": "Allow",
    "drop": "Drop",
    "empty": "No traffic audit events",
    "noMatch": "No matching events found",
    "error": "Failed to load audit events"
  }
}
```

- [ ] **Step 3: Commit**

```bash
git add fronted/src/locales/zh-CN/common.json fronted/src/locales/en/common.json
git commit -s -m "feat(frontend): add sandbox i18n keys for navigation and pages"
```

---

### Task 3: Rewrite Home Page (pages/index.vue)

**Files:**
- Modify: `fronted/src/pages/index.vue`

This is a full rewrite of the Home page. The existing page is ~594 lines. The new version replaces all section content while keeping the same structural shell (`blank` layout, navbar, footer).

- [ ] **Step 1: Rewrite index.vue — script section**

Keep the script unchanged (router, i18n, userStore, interval timer for latency). The only change needed: update section anchor IDs in navbar and remove unused icon imports (`Shield`, `Layers`, `Zap`, `Globe`, `Terminal`, `Lock`, `Crown` no longer needed in advantages/nav — but `Crown` stays for pricing, `Zap` stays for CTA). No script logic changes required beyond import cleanup.

Replace the entire `<script setup>` block — imports simplified to only what we need:

```typescript
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  ArrowRight, Cpu, Zap, Globe, Shield,
  CheckCircle, ChevronRight, Crown, X, LogOut, LayoutDashboard, Container,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { storeToRefs } from 'pinia'
import { useUserStore } from '@/stores/user'

definePage({ meta: { layout: 'blank' } })

const { t } = useI18n()
const router = useRouter()
const userStore = useUserStore()
const { userInfo } = storeToRefs(userStore)
const { logout } = userStore

const avatarFallback = computed(() => {
  const name = userInfo.value?.username ?? userInfo.value?.email ?? '?'
  return name.slice(0, 2).toUpperCase()
})

const latency = ref(42)
const lastSync = ref('')
let timer: ReturnType<typeof setInterval>

onMounted(() => {
  lastSync.value = new Date().toLocaleTimeString([], { hour12: false })
  timer = setInterval(() => {
    latency.value = Math.floor(Math.random() * 8) + 38
    lastSync.value = new Date().toLocaleTimeString([], { hour12: false })
  }, 3000)
})
onUnmounted(() => clearInterval(timer))
</script>
```

- [ ] **Step 2: Rewrite the template — sections 1-3: Navbar, Hero, Terminal**

Keep the navbar structure identical (just update nav links if needed — `#features`, `#architecture`, `#pricing`, `#quickstart`). The Hero section replaces title/subtitle/CTA text with i18n keys from `landing.hero.*`. The terminal mockup replaces content with i18n keys from `landing.terminal.*`.

The terminal mockup HTML structure stays the same (title bar with red/yellow/green dots, 3-column stats row). Only content changes via i18n:

```html
<!-- Navbar: unchanged structure, same anchor IDs -->
<!-- Hero -->
<section class="relative overflow-hidden pt-24 pb-20 px-6">
  <!-- grid background SVG: unchanged -->
  <!-- network topology SVG: unchanged -->
  <!-- glow: unchanged -->
  <div class="max-w-3xl mx-auto text-center relative">
    <div class="inline-flex items-center gap-2 px-3 py-1.5 mb-8 rounded-full border border-border bg-muted text-xs font-medium text-muted-foreground">
      <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
      {{ t('landing.hero.badge') }}
    </div>
    <h1 class="text-4xl md:text-[3.5rem] font-black tracking-tighter leading-[1.1] mb-5 bg-gradient-to-r from-gray-900 via-indigo-600 to-cyan-500 bg-clip-text text-transparent dark:from-gray-100 dark:via-indigo-400 dark:to-cyan-300">
      {{ t('landing.hero.title') }}
    </h1>
    <p class="text-muted-foreground text-base leading-relaxed max-w-xl mx-auto mb-8">
      {{ t('landing.hero.subtitle') }}
    </p>
    <div class="flex flex-col sm:flex-row items-center justify-center gap-3">
      <Button size="lg" class="gap-2 px-7 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white border-0 shadow-lg shadow-indigo-500/20" @click="router.push('/manage/stepper')">
        <Zap class="size-4" /> {{ t('landing.hero.cta_primary') }}
      </Button>
      <Button variant="outline" size="lg" class="gap-2 px-7 border-border" @click="router.push('/dashboard')">
        {{ t('landing.hero.cta_secondary') }} <ChevronRight class="size-4" />
      </Button>
    </div>
  </div>
</section>

<!-- Terminal: same structure, i18n content -->
<section class="px-6 pb-20">
  <div class="max-w-3xl mx-auto">
    <div class="rounded-2xl overflow-hidden border border-indigo-950 shadow-xl shadow-indigo-950/20 bg-gradient-to-b from-[#0f0d2e] to-[#1a1740]">
      <div class="flex items-center gap-1.5 px-4 py-2.5 bg-[#1e1b4b] border-b border-indigo-950">
        <div class="size-3 rounded-full bg-rose-500/70" />
        <div class="size-3 rounded-full bg-amber-400/70" />
        <div class="size-3 rounded-full bg-emerald-500/70" />
        <span class="ml-2 text-[11px] text-indigo-300/60 font-mono flex-1">{{ t('landing.terminal.title') }}</span>
        <div class="flex items-center gap-1.5">
          <span class="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
          <span class="text-[11px] text-emerald-400 font-mono font-semibold">{{ t('landing.terminal.status') }}</span>
        </div>
      </div>
      <div class="p-5 font-mono text-sm leading-7">
        <p><span class="text-indigo-300/30 select-none">#  </span><span class="text-indigo-300/50">{{ t('landing.terminal.line1') }}</span></p>
        <p><span class="text-emerald-400/60">{{ t('landing.terminal.line2') }}</span></p>
        <p><span class="text-emerald-400/60">{{ t('landing.terminal.line3') }}</span></p>
        <p><span class="text-emerald-400/60">{{ t('landing.terminal.line4') }}</span></p>
        <p class="mt-1"><span class="text-sky-400">{{ t('landing.terminal.line5') }}</span></p>
        <p class="mt-3"><span class="text-indigo-300/30 select-none">#  </span><span class="text-indigo-300/50 italic">{{ t('landing.terminal.line6') }}</span></p>
        <p><span class="text-indigo-300/30 select-none">$  </span><span class="text-white">{{ t('landing.terminal.line7') }}</span></p>
        <p><span class="text-emerald-400/60">{{ t('landing.terminal.line8') }}</span></p>
        <p><span class="text-emerald-400/60">{{ t('landing.terminal.line9') }}</span></p>
        <p class="mt-3"><span class="text-indigo-300/30 select-none">#  </span><span class="text-rose-400/70 italic">{{ t('landing.terminal.line10') }}</span></p>
        <p><span class="text-indigo-300/30 select-none">$  </span><span class="text-white">{{ t('landing.terminal.line11') }}</span></p>
        <p><span class="text-rose-400/60">{{ t('landing.terminal.line12') }}</span></p>
        <p><span class="text-rose-400/60">{{ t('landing.terminal.line13') }}</span></p>
      </div>
    </div>
  </div>
</section>
```

- [ ] **Step 3: Features section (replaces old features + advantages)**

Two rows of 3 cards in a single grid. Remove the separate "advantages" section (6-item grid below old features) — its content is now folded into the terminal mockup and features.

```html
<section id="features" class="py-20 px-6 bg-muted/50 border-y border-border">
  <div class="max-w-5xl mx-auto">
    <div class="text-center mb-12">
      <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-2">{{ t('landing.features.label') }}</p>
      <h2 class="text-2xl font-black tracking-tighter text-foreground">{{ t('landing.features.title') }}</h2>
      <p class="text-muted-foreground text-sm mt-2.5 max-w-lg mx-auto leading-relaxed">{{ t('landing.features.subtitle') }}</p>
    </div>

    <!-- Row 1: AI Agent capabilities -->
    <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-4">AI Agent 运行时</p>
    <div class="grid md:grid-cols-3 gap-4 mb-8">
      <div class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 hover:-translate-y-0.5 transition-all duration-200">
        <div class="size-10 rounded-xl bg-violet-50 dark:bg-violet-500/10 text-violet-600 dark:text-violet-400 flex items-center justify-center mb-4">
          <Container class="size-5" />
        </div>
        <span class="text-[10px] font-bold px-2 py-0.5 rounded-full text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-500/10 ring-1 ring-emerald-200 dark:ring-emerald-500/20">{{ t('landing.features.tag_stable') }}</span>
        <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t('landing.features.ai_sandbox.title') }}</h3>
        <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.features.ai_sandbox.desc') }}</p>
      </div>

      <div class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 hover:-translate-y-0.5 transition-all duration-200">
        <div class="size-10 rounded-xl bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 flex items-center justify-center mb-4">
          <Shield class="size-5" />
        </div>
        <span class="text-[10px] font-bold px-2 py-0.5 rounded-full text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-500/10 ring-1 ring-emerald-200 dark:ring-emerald-500/20">{{ t('landing.features.tag_stable') }}</span>
        <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t('landing.features.ai_sidecar.title') }}</h3>
        <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.features.ai_sidecar.desc') }}</p>
      </div>

      <div class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 hover:-translate-y-0.5 transition-all duration-200">
        <div class="size-10 rounded-xl bg-amber-50 dark:bg-amber-500/10 text-amber-600 dark:text-amber-400 flex items-center justify-center mb-4">
          <Zap class="size-5" />
        </div>
        <span class="text-[10px] font-bold px-2 py-0.5 rounded-full text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-500/10 ring-1 ring-amber-200 dark:ring-amber-400/20">{{ t('landing.features.tag_roadmap') }}</span>
        <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t('landing.features.ai_intent.title') }}</h3>
        <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.features.ai_intent.desc') }}</p>
      </div>
    </div>

    <!-- Row 2: Network foundation -->
    <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-4">网络基础层</p>
    <div class="grid md:grid-cols-3 gap-4">
      <div class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 hover:-translate-y-0.5 transition-all duration-200">
        <div class="size-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center mb-4">
          <Globe class="size-5" />
        </div>
        <span class="text-[10px] font-bold px-2 py-0.5 rounded-full text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-500/10 ring-1 ring-emerald-200 dark:ring-emerald-500/20">{{ t('landing.features.tag_stable') }}</span>
        <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t('landing.features.net_wg.title') }}</h3>
        <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.features.net_wg.desc') }}</p>
      </div>

      <div class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 hover:-translate-y-0.5 transition-all duration-200">
        <div class="size-10 rounded-xl bg-amber-50 dark:bg-amber-500/10 text-amber-600 dark:text-amber-400 flex items-center justify-center mb-4">
          <Cpu class="size-5" />
        </div>
        <span class="text-[10px] font-bold px-2 py-0.5 rounded-full text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-500/10 ring-1 ring-amber-200 dark:ring-amber-400/20">{{ t('landing.features.tag_roadmap') }}</span>
        <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t('landing.features.net_ebpf.title') }}</h3>
        <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.features.net_ebpf.desc') }}</p>
      </div>

      <div class="bg-card border border-border rounded-xl p-6 hover:shadow-md hover:border-border/60 hover:-translate-y-0.5 transition-all duration-200">
        <div class="size-10 rounded-xl bg-cyan-50 dark:bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 flex items-center justify-center mb-4">
          <Shield class="size-5" />
        </div>
        <span class="text-[10px] font-bold px-2 py-0.5 rounded-full text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-500/10 ring-1 ring-emerald-200 dark:ring-emerald-500/20">{{ t('landing.features.tag_stable') }}</span>
        <h3 class="text-sm font-bold mt-3 mb-1.5 text-card-foreground">{{ t('landing.features.net_audit.title') }}</h3>
        <p class="text-xs text-muted-foreground leading-relaxed">{{ t('landing.features.net_audit.desc') }}</p>
      </div>
    </div>
  </div>
</section>
```

- [ ] **Step 4: Architecture section (replaces old IaC section)**

Keep the same split layout (left steps + right terminal), replace content with sandbox lifecycle:

```html
<section id="architecture" class="py-20 px-6 bg-muted/50 border-y border-border">
  <div class="max-w-5xl mx-auto">
    <div class="text-center mb-12">
      <p class="text-[10px] font-black uppercase tracking-widest text-muted-foreground mb-2">{{ t('landing.architecture.label') }}</p>
      <h2 class="text-2xl font-black tracking-tighter text-foreground">{{ t('landing.architecture.title') }}</h2>
      <p class="text-muted-foreground text-sm mt-2.5 max-w-md mx-auto">{{ t('landing.architecture.subtitle') }}</p>
    </div>
    <div class="flex flex-col lg:flex-row gap-5">
      <div class="lg:w-2/5 bg-card border border-border rounded-xl p-6">
        <div class="space-y-0">
          <div class="flex items-start gap-3.5 relative">
            <div class="flex flex-col items-center">
              <div class="size-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center text-[11px] font-black shrink-0">01</div>
              <div class="w-px flex-1 bg-border min-h-[2.5rem]" />
            </div>
            <div class="pb-5">
              <p class="text-sm font-semibold text-card-foreground">{{ t('landing.architecture.step_1_title') }}</p>
              <p class="text-xs text-muted-foreground mt-0.5 leading-relaxed">{{ t('landing.architecture.step_1_desc') }}</p>
            </div>
          </div>
          <div class="flex items-start gap-3.5 relative">
            <div class="flex flex-col items-center">
              <div class="size-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center text-[11px] font-black shrink-0">02</div>
              <div class="w-px flex-1 bg-border min-h-[2.5rem]" />
            </div>
            <div class="pb-5">
              <p class="text-sm font-semibold text-card-foreground">{{ t('landing.architecture.step_2_title') }}</p>
              <p class="text-xs text-muted-foreground mt-0.5 leading-relaxed">{{ t('landing.architecture.step_2_desc') }}</p>
            </div>
          </div>
          <div class="flex items-start gap-3.5 relative">
            <div class="flex flex-col items-center">
              <div class="size-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center text-[11px] font-black shrink-0">03</div>
            </div>
            <div>
              <p class="text-sm font-semibold text-card-foreground">{{ t('landing.architecture.step_3_title') }}</p>
              <p class="text-xs text-muted-foreground mt-0.5 leading-relaxed">{{ t('landing.architecture.step_3_desc') }}</p>
            </div>
          </div>
        </div>
      </div>
      <div class="lg:w-3/5 rounded-xl overflow-hidden border border-[#1e1b4b] bg-[#0f0d2e]">
        <div class="flex items-center gap-1.5 px-4 py-2.5 bg-[#1e1b4b] border-b border-[#1e1b4b]">
          <div class="size-2.5 rounded-full bg-rose-500/70" />
          <div class="size-2.5 rounded-full bg-amber-400/70" />
          <div class="size-2.5 rounded-full bg-emerald-500/70" />
          <span class="ml-2 text-[11px] text-indigo-300/60 font-mono">bash</span>
        </div>
        <div class="p-5 font-mono text-sm leading-7">
          <p><span class="text-indigo-300/30 select-none">#  </span><span class="text-indigo-300/50 italic">{{ t('landing.architecture.terminal_comment') }}</span></p>
          <p><span class="text-indigo-300/50">{{ t('landing.architecture.terminal_line1') }}</span></p>
          <p><span class="text-emerald-400/60">{{ t('landing.architecture.terminal_line2') }}</span></p>
          <p><span class="text-emerald-400/60">{{ t('landing.architecture.terminal_line3') }}</span></p>
          <p><span class="text-emerald-400/60">{{ t('landing.architecture.terminal_line4') }}</span></p>
          <p><span class="text-emerald-400/60">{{ t('landing.architecture.terminal_line5') }}</span></p>
          <p class="mt-3"><span class="text-indigo-300/30 select-none">#  </span><span class="text-rose-400/70 italic">{{ t('landing.architecture.terminal_comment2') }}</span></p>
          <p><span class="text-rose-400/60">{{ t('landing.architecture.terminal_line6') }}</span></p>
          <p><span class="text-rose-400/60">{{ t('landing.architecture.terminal_line7') }}</span></p>
          <p><span class="text-rose-400/60">{{ t('landing.architecture.terminal_line8') }}</span></p>
          <p><span class="text-rose-400/60">{{ t('landing.architecture.terminal_line9') }}</span></p>
        </div>
      </div>
    </div>
  </div>
</section>
```

- [ ] **Step 5: Pricing section (updated feature lists)**

The pricing section keeps the same two-column layout. Only the feature list items change — replace content with i18n keys referencing the new `landing.pricing.*` keys. The Community column gets 7 features + 3 locked. The Pro column gets "all community features" + 8 Pro features.

Key changes from old to new:

Community column (replace all 7 li items):
```html
<li class="flex items-center gap-2.5 text-sm text-foreground">
  <CheckCircle class="size-4 text-emerald-500 shrink-0" />
  {{ t('landing.pricing.community_feat_1') }}
</li>
<!-- ... through feat_7 ... -->
<li class="flex items-center gap-2.5 text-sm text-muted-foreground/50 line-through">
  <X class="size-4 text-muted-foreground/30 shrink-0" />
  {{ t('landing.pricing.pro_feat_locked_1') }}
</li>
<!-- ... through locked_3 ... -->
```

Pro column (replace all 7 + 1 items with "all community" + 8):
```html
<li class="flex items-center gap-2.5 text-sm font-medium text-primary">
  <CheckCircle class="size-4 shrink-0" />
  {{ t('landing.pricing.pro_feat_all') }}
</li>
<li class="flex items-center gap-2.5 text-sm text-foreground">
  <CheckCircle class="size-4 text-emerald-500 shrink-0" />
  {{ t('landing.pricing.pro_feat_1') }}
</li>
<!-- ... through feat_8 ... -->
```

- [ ] **Step 6: CTA section (updated text and badges)**

```html
<section id="quickstart" class="py-20 px-6">
  <div class="max-w-xl mx-auto text-center">
    <div class="size-14 rounded-2xl bg-gradient-to-br from-indigo-600/10 to-cyan-500/10 flex items-center justify-center mx-auto mb-5">
      <Container class="size-7 text-indigo-500" />
    </div>
    <h2 class="text-2xl font-black tracking-tighter mb-3 text-foreground">{{ t('landing.cta.title') }}</h2>
    <p class="text-muted-foreground text-sm leading-relaxed mb-7 max-w-sm mx-auto">{{ t('landing.cta.subtitle') }}</p>
    <div class="flex flex-col sm:flex-row gap-3 justify-center mb-8">
      <Button size="lg" class="gap-2 px-8 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white border-0 shadow-lg shadow-indigo-500/20" @click="router.push('/manage/stepper')">
        <Zap class="size-4" /> {{ t('landing.cta.button_primary') }}
      </Button>
      <Button variant="outline" size="lg" class="gap-2 px-8 border-border" @click="router.push('/dashboard')">
        {{ t('landing.cta.button_secondary') }} <ArrowRight class="size-4" />
      </Button>
    </div>
    <div class="grid grid-cols-3 gap-2 text-left max-w-xs mx-auto">
      <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_1') }}</div>
      <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_2') }}</div>
      <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_3') }}</div>
      <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_4') }}</div>
      <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_5') }}</div>
      <div class="flex items-center gap-1.5 text-xs text-muted-foreground"><CheckCircle class="size-3.5 text-emerald-500 shrink-0" />{{ t('landing.cta.badge_6') }}</div>
    </div>
  </div>
</section>
```

- [ ] **Step 7: Footer — keep unchanged**

No changes to the footer from the original.

- [ ] **Step 8: Commit**

```bash
git add fronted/src/pages/index.vue
git commit -s -m "feat(frontend): rewrite Home page with AI agent sandbox narrative"
```

---

### Task 4: Add Agent Sandbox Navigation to Sidebar

**Files:**
- Modify: `fronted/src/components/app-sidebar/AppSidebar.vue`

- [ ] **Step 1: Add sandbox icon import and navigation group**

Add `Container` to the lucide-vue-next import on line 12:

Change:
```typescript
import {
  LayoutDashboard, Network, Settings2,
  ShieldCheck, Bot, House,
} from "lucide-vue-next"
```

To:
```typescript
import {
  LayoutDashboard, Network, Settings2,
  ShieldCheck, Bot, House, Container,
} from "lucide-vue-next"
```

Add the sandbox navigation group in the `navMain` computed, after the AI Assistant group and before Settings:

```typescript
// ── Agent Sandbox ──────────────────────────────────────────────
{
  title: t('common.nav.group.sandbox'),
  url: '/sandbox',
  icon: Container,
  items: [
    { title: t('common.nav.sandboxList'),  url: '/sandbox' },
    { title: t('common.nav.sandboxTokens'), url: '/sandbox/tokens' },
    { title: t('common.nav.sandboxAudit'),  url: '/sandbox/audit' },
  ],
},
```

- [ ] **Step 2: Commit**

```bash
git add fronted/src/components/app-sidebar/AppSidebar.vue
git commit -s -m "feat(frontend): add Agent Sandbox navigation group to sidebar"
```

---

### Task 5: Add Sandbox Stat Card to Dashboard

**Files:**
- Modify: `fronted/src/pages/dashboard/index.vue` (lines 33-44: update `iconByIndex`, `titleKeyByIndex`, `colorByIndex` arrays from 4 to 5 elements)

- [ ] **Step 1: Update the icon/color/title arrays for 5 cards**

In the dashboard page script section, extend the three arrays from 4 to 5 elements:

```typescript
const iconByIndex = [Server, Activity, ShieldCheck, AlertTriangle, Container]
const titleKeyByIndex = [
  'settings.dashboard.statNodes',
  'settings.dashboard.statTunnels',
  'settings.dashboard.statPolicies',
  'settings.dashboard.statAlerts',
  'settings.dashboard.statSandboxes',
]
const colorByIndex = [
  { badge: 'bg-blue-500/10',    icon: 'text-blue-500',    num: 'text-blue-600 dark:text-blue-400' },
  { badge: 'bg-emerald-500/10', icon: 'text-emerald-500', num: 'text-emerald-600 dark:text-emerald-400' },
  { badge: 'bg-primary/10',     icon: 'text-primary',     num: 'text-primary' },
  { badge: 'bg-amber-500/10',   icon: 'text-amber-500',   num: 'text-amber-600 dark:text-amber-400' },
  { badge: 'bg-violet-500/10',  icon: 'text-violet-500',  num: 'text-violet-600 dark:text-violet-400' },
]
```

Add `Container` to the import from lucide-vue-next if not already available.

- [ ] **Step 2: Update stat card grid for 5 columns**

Change `xl:grid-cols-4` to `xl:grid-cols-5` on the stat cards grid:

```html
<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
```

- [ ] **Step 3: Add statSandboxes i18n key**

Add to the `settings` translations files (both en and zh-CN):

CN `settings.json`:
```json
"statSandboxes": "Agent 沙箱"
```

EN `settings.json`:
```json
"statSandboxes": "Agent Sandboxes"
```

- [ ] **Step 4: Commit**

```bash
git add fronted/src/pages/dashboard/index.vue fronted/src/locales/zh-CN/settings.json fronted/src/locales/en/settings.json
git commit -s -m "feat(frontend): add sandbox stat card to dashboard"
```

---

### Task 6: Sandbox API Client

**Files:**
- Create: `fronted/src/api/sandbox.ts`

- [ ] **Step 1: Write sandbox API module**

```typescript
import request from '@/api/request'

function wsID(): string {
  return localStorage.getItem('active_ws_id') || ''
}

// ── Types ────────────────────────────────────────────────────────

export interface SandboxAgent {
  name: string
  sandboxId: string
  mode: 'gvisor' | 'cgroup'
  status: 'online' | 'offline'
  vpnIP: string
  publicKey: string
  trafficRx: number
  trafficTx: number
  createdAt: string
}

export interface EnrollmentToken {
  token?: string
  maskedToken: string
  expiresAt: string
  createdAt: string
  status: 'active' | 'expired' | 'revoked'
  allowedTools: string[]
}

export interface CreateTokenInput {
  allowedTools: string[]
  ttlSeconds: number
}

export interface TrafficAuditEvent {
  id: string
  timestamp: string
  sandboxName: string
  srcIP: string
  dstIP: string
  dstPort: number
  protocol: 'tcp' | 'udp'
  verdict: 'allow' | 'drop'
  detail?: string
}

export interface TrafficAuditParams {
  keyword?: string
  verdict?: 'allow' | 'drop' | ''
  from?: string
  to?: string
  page?: number
  pageSize?: number
}

// ── API functions ────────────────────────────────────────────────

export const listSandboxes = (): Promise<SandboxAgent[]> =>
  request.get(`/agent-isolation/agents?workspace=${wsID()}`)

export const revokeSandbox = (name: string): Promise<void> =>
  request.delete(`/agent-isolation/agents/${name}?workspace=${wsID()}`)

export const listTokens = (): Promise<EnrollmentToken[]> =>
  request.get(`/agent-isolation/enrollment-tokens?workspace=${wsID()}`)

export const createToken = (input: CreateTokenInput): Promise<EnrollmentToken> =>
  request.post('/agent-isolation/enrollment-tokens', { ...input, namespace: wsID() })

export const revokeToken = (token: string): Promise<void> =>
  request.delete(`/agent-isolation/enrollment-tokens/${token}?workspace=${wsID()}`)

export const listTrafficAudit = (params: TrafficAuditParams = {}): Promise<TrafficAuditEvent[]> =>
  request.get(`/workspaces/${wsID()}/audit-logs`, { ...params, type: 'traffic' })
```

- [ ] **Step 2: Commit**

```bash
git add fronted/src/api/sandbox.ts
git commit -s -m "feat(frontend): add sandbox API client module"
```

---

### Task 7: Sandbox Store

**Files:**
- Create: `fronted/src/stores/useSandboxStore.ts`

- [ ] **Step 1: Write useSandboxStore**

```typescript
import { defineStore } from 'pinia'
import {
  listSandboxes, revokeSandbox,
  listTokens, createToken, revokeToken,
  listTrafficAudit,
  type SandboxAgent, type EnrollmentToken, type CreateTokenInput,
  type TrafficAuditEvent, type TrafficAuditParams,
} from '@/api/sandbox'

export const useSandboxStore = defineStore('sandbox', {
  state: () => ({
    sandboxes: [] as SandboxAgent[],
    sandboxesLoading: false,
    sandboxesError: null as string | null,

    tokens: [] as EnrollmentToken[],
    tokensLoading: false,
    tokensError: null as string | null,

    auditEvents: [] as TrafficAuditEvent[],
    auditLoading: false,
    auditError: null as string | null,
  }),

  getters: {
    onlineCount: (state) => state.sandboxes.filter(s => s.status === 'online').length,
    totalSandboxes: (state) => state.sandboxes.length,
  },

  actions: {
    async fetchSandboxes() {
      this.sandboxesLoading = true
      this.sandboxesError = null
      try {
        this.sandboxes = await listSandboxes()
      } catch (e: any) {
        this.sandboxesError = e?.message || 'Failed to load sandboxes'
      } finally {
        this.sandboxesLoading = false
      }
    },

    async revokeSandbox(name: string) {
      await revokeSandbox(name)
      this.sandboxes = this.sandboxes.filter(s => s.name !== name)
    },

    async fetchTokens() {
      this.tokensLoading = true
      this.tokensError = null
      try {
        this.tokens = await listTokens()
      } catch (e: any) {
        this.tokensError = e?.message || 'Failed to load tokens'
      } finally {
        this.tokensLoading = false
      }
    },

    async generateToken(input: CreateTokenInput): Promise<EnrollmentToken> {
      const token = await createToken(input)
      this.tokens.unshift(token)
      return token
    },

    async revokeToken(token: string) {
      await revokeToken(token)
      this.tokens = this.tokens.filter(t => t.maskedToken !== token && t.token !== token)
    },

    async fetchAudit(params: TrafficAuditParams = {}) {
      this.auditLoading = true
      this.auditError = null
      try {
        this.auditEvents = await listTrafficAudit(params)
      } catch (e: any) {
        this.auditError = e?.message || 'Failed to load audit events'
      } finally {
        this.auditLoading = false
      }
    },
  },
})
```

- [ ] **Step 2: Commit**

```bash
git add fronted/src/stores/useSandboxStore.ts
git commit -s -m "feat(frontend): add sandbox Pinia store"
```

---

### Task 8: Sandbox List Page

**Files:**
- Create: `fronted/src/pages/sandbox/index.vue`

- [ ] **Step 1: Write sandbox list page**

```vue
<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Container, Loader2, Trash2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { useSandboxStore } from '@/stores/useSandboxStore'
import { toast } from 'vue-sonner'

definePage({
  meta: { titleKey: 'sandbox.list.title', descKey: 'sandbox.list.desc' },
})

const { t } = useI18n()
const store = useSandboxStore()

onMounted(() => store.fetchSandboxes())

const statusClass = (status: string) =>
  status === 'online'
    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
    : 'bg-muted text-muted-foreground'

const modeLabel = (mode: string) =>
  mode === 'gvisor' ? t('sandbox.list.modeGvisor') : t('sandbox.list.modeCgroup')

function formatBytes(bytes: number): string {
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`
  if (bytes >= 1e6) return `${(bytes / 1e6).toFixed(1)} MB`
  if (bytes >= 1e3) return `${(bytes / 1e3).toFixed(1)} KB`
  return `${bytes} B`
}

async function handleRevoke(name: string) {
  if (!confirm(`${t('sandbox.list.confirmRevokeDesc')}`)) return
  try {
    await store.revokeSandbox(name)
    toast.success(`${name} ${t('sandbox.list.revoke')}`)
  } catch (e: any) {
    toast.error(e?.message || t('sandbox.list.error'))
  }
}
</script>

<template>
  <div class="p-6 space-y-4">
    <div v-if="store.sandboxesLoading" class="flex gap-4">
      <div v-for="i in 3" :key="i" class="h-16 flex-1 animate-pulse rounded-xl bg-muted" />
    </div>

    <div v-else-if="store.sandboxes.length === 0" class="flex flex-col items-center gap-2 py-16 text-sm text-muted-foreground">
      <Container class="size-10 opacity-40" />
      <p>{{ t('sandbox.list.empty') }}</p>
    </div>

    <div v-else class="rounded-xl border border-border bg-card overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('sandbox.list.colName') }}</TableHead>
            <TableHead>{{ t('sandbox.list.colStatus') }}</TableHead>
            <TableHead>{{ t('sandbox.list.colMode') }}</TableHead>
            <TableHead>{{ t('sandbox.list.colIP') }}</TableHead>
            <TableHead>{{ t('sandbox.list.colSandboxId') }}</TableHead>
            <TableHead>{{ t('sandbox.list.colTrafficRx') }}</TableHead>
            <TableHead>{{ t('sandbox.list.colTrafficTx') }}</TableHead>
            <TableHead>{{ t('sandbox.list.colCreated') }}</TableHead>
            <TableHead>{{ t('sandbox.list.colActions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="s in store.sandboxes" :key="s.name">
            <TableCell class="font-medium">{{ s.name }}</TableCell>
            <TableCell>
              <Badge :class="statusClass(s.status)" variant="secondary">
                {{ t(`sandbox.list.${s.status}`) }}
              </Badge>
            </TableCell>
            <TableCell class="text-sm">{{ modeLabel(s.mode) }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">{{ s.vpnIP }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">{{ s.sandboxId }}</TableCell>
            <TableCell class="text-sm">{{ formatBytes(s.trafficRx) }}</TableCell>
            <TableCell class="text-sm">{{ formatBytes(s.trafficTx) }}</TableCell>
            <TableCell class="text-xs text-muted-foreground">{{ new Date(s.createdAt).toLocaleString() }}</TableCell>
            <TableCell>
              <Button variant="ghost" size="icon" class="size-8 text-muted-foreground hover:text-destructive" @click="handleRevoke(s.name)">
                <Trash2 class="size-4" />
              </Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add fronted/src/pages/sandbox/index.vue
git commit -s -m "feat(frontend): add sandbox list page"
```

---

### Task 9: Enrollment Tokens Page

**Files:**
- Create: `fronted/src/pages/sandbox/tokens.vue`

- [ ] **Step 1: Write tokens page**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Key, Loader2, Copy, Check, X, Plus, Clock } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { useSandboxStore } from '@/stores/useSandboxStore'
import type { EnrollmentToken } from '@/api/sandbox'
import { toast } from 'vue-sonner'

definePage({
  meta: { titleKey: 'sandbox.tokens.title', descKey: 'sandbox.tokens.desc' },
})

const { t } = useI18n()
const store = useSandboxStore()

const ttlOptions = [
  { value: 3600, label: 'sandbox.tokens.ttl1h' },
  { value: 21600, label: 'sandbox.tokens.ttl6h' },
  { value: 86400, label: 'sandbox.tokens.ttl24h' },
]

const toolOptions = [
  { value: 'list_peers', label: 'list_peers' },
  { value: 'list_policies', label: 'list_policies' },
  { value: 'check_connectivity', label: 'check_connectivity' },
  { value: 'list_networks', label: 'list_networks' },
]

const ttl = ref(3600)
const allowedTools = ref<string[]>(['list_peers', 'list_policies'])
const creating = ref(false)

const generatedToken = ref<EnrollmentToken | null>(null)
const tokenDialog = ref(false)
const copied = ref(false)

function toggleTool(tool: string) {
  const idx = allowedTools.value.indexOf(tool)
  if (idx >= 0) allowedTools.value.splice(idx, 1)
  else allowedTools.value.push(tool)
}

async function handleCreate() {
  creating.value = true
  try {
    const result = await store.generateToken({
      allowedTools: allowedTools.value,
      ttlSeconds: ttl.value,
    })
    generatedToken.value = result
    tokenDialog.value = true
  } catch (e: any) {
    toast.error(e?.message || t('sandbox.tokens.error'))
  } finally {
    creating.value = false
  }
}

async function copyToken() {
  if (!generatedToken.value?.token) return
  await navigator.clipboard.writeText(generatedToken.value.token)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

function closeTokenDialog() {
  tokenDialog.value = false
  generatedToken.value = null
  copied.value = false
}

async function handleRevoke(token: string) {
  try {
    await store.revokeToken(token)
    toast.success(t('sandbox.tokens.revoke'))
  } catch (e: any) {
    toast.error(e?.message || t('sandbox.tokens.error'))
  }
}

const statusClass = (status: string) => {
  const map: Record<string, string> = {
    active: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
    expired: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
    revoked: 'bg-muted text-muted-foreground',
  }
  return map[status] || map.revoked
}

onMounted(() => store.fetchTokens())
</script>

<template>
  <div class="p-6 space-y-6">
    <!-- Create section -->
    <div class="rounded-xl border border-border bg-card p-6">
      <h3 class="mb-1 font-semibold">{{ t('sandbox.tokens.createTitle') }}</h3>
      <p class="text-muted-foreground mb-4 text-sm">{{ t('sandbox.tokens.createDesc') }}</p>

      <div class="space-y-4">
        <!-- TTL -->
        <div>
          <label class="text-sm font-medium">{{ t('sandbox.tokens.ttl') }}</label>
          <div class="flex gap-2 mt-1.5">
            <Button
              v-for="opt in ttlOptions" :key="opt.value"
              :variant="ttl === opt.value ? 'default' : 'outline'"
              size="sm"
              @click="ttl = opt.value"
            >
              {{ t(opt.label) }}
            </Button>
          </div>
        </div>

        <!-- Allowed Tools -->
        <div>
          <label class="text-sm font-medium">{{ t('sandbox.tokens.allowedTools') }}</label>
          <div class="flex flex-wrap gap-2 mt-1.5">
            <button
              v-for="tool in toolOptions" :key="tool.value"
              class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs transition-colors"
              :class="allowedTools.includes(tool.value)
                ? 'bg-primary/10 border-primary/30 text-primary'
                : 'border-border text-muted-foreground hover:bg-muted'"
              @click="toggleTool(tool.value)"
            >
              {{ tool.label }}
              <Check v-if="allowedTools.includes(tool.value)" class="size-3" />
            </button>
          </div>
        </div>

        <Button :disabled="creating || allowedTools.length === 0" @click="handleCreate">
          <Loader2 v-if="creating" class="mr-2 size-4 animate-spin" />
          <Plus v-else class="mr-2 size-4" />
          {{ t('sandbox.tokens.generate') }}
        </Button>
      </div>
    </div>

    <!-- Token list -->
    <div v-if="store.tokensLoading" class="flex gap-4">
      <div v-for="i in 3" :key="i" class="h-12 flex-1 animate-pulse rounded-lg bg-muted" />
    </div>

    <div v-else-if="store.tokens.length === 0" class="flex flex-col items-center gap-2 py-12 text-sm text-muted-foreground">
      <Key class="size-10 opacity-40" />
      <p>{{ t('sandbox.tokens.empty') }}</p>
    </div>

    <div v-else class="rounded-xl border border-border bg-card overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('sandbox.tokens.colToken') }}</TableHead>
            <TableHead>{{ t('sandbox.tokens.colCreated') }}</TableHead>
            <TableHead>{{ t('sandbox.tokens.colExpires') }}</TableHead>
            <TableHead>{{ t('sandbox.tokens.colStatus') }}</TableHead>
            <TableHead>{{ t('sandbox.tokens.colActions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="tok in store.tokens" :key="tok.maskedToken">
            <TableCell class="font-mono text-xs">{{ tok.maskedToken }}</TableCell>
            <TableCell class="text-xs text-muted-foreground">{{ new Date(tok.createdAt).toLocaleString() }}</TableCell>
            <TableCell class="text-xs text-muted-foreground">{{ new Date(tok.expiresAt).toLocaleString() }}</TableCell>
            <TableCell>
              <Badge :class="statusClass(tok.status)" variant="secondary">
                {{ t(`sandbox.tokens.status${tok.status.charAt(0).toUpperCase() + tok.status.slice(1)}`) }}
              </Badge>
            </TableCell>
            <TableCell>
              <Button
                v-if="tok.status === 'active'"
                variant="ghost" size="sm"
                class="text-destructive hover:text-destructive text-xs"
                @click="handleRevoke(tok.token || tok.maskedToken)"
              >
                <X class="mr-1 size-3" />
                {{ t('sandbox.tokens.revoke') }}
              </Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- Token dialog -->
    <Dialog :open="tokenDialog" @update:open="closeTokenDialog">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('sandbox.tokens.generatedToken') }}</DialogTitle>
          <DialogDescription class="text-amber-600 dark:text-amber-400">
            {{ t('sandbox.tokens.tokenWarning') }}
          </DialogDescription>
        </DialogHeader>
        <div class="flex items-center gap-2 rounded-lg bg-muted p-3 font-mono text-sm break-all">
          <code class="flex-1 text-xs">{{ generatedToken?.token }}</code>
          <Button variant="ghost" size="icon" class="shrink-0 size-8" @click="copyToken">
            <Check v-if="copied" class="size-4 text-emerald-500" />
            <Copy v-else class="size-4" />
          </Button>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="closeTokenDialog">{{ t('common.action.close') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add fronted/src/pages/sandbox/tokens.vue
git commit -s -m "feat(frontend): add enrollment tokens management page"
```

---

### Task 10: Traffic Audit Page

**Files:**
- Create: `fronted/src/pages/sandbox/audit.vue`

- [ ] **Step 1: Write audit page**

```vue
<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Search, ShieldCheck, Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { useSandboxStore } from '@/stores/useSandboxStore'

definePage({
  meta: { titleKey: 'sandbox.audit.title', descKey: 'sandbox.audit.desc' },
})

const { t } = useI18n()
const store = useSandboxStore()

const searchQuery = ref('')
const verdictFilter = ref<'allow' | 'drop' | ''>('')

const filteredEvents = computed(() => {
  let events = store.auditEvents
  const q = searchQuery.value.toLowerCase().trim()
  if (q) {
    events = events.filter(e =>
      e.sandboxName.toLowerCase().includes(q) ||
      e.dstIP.toLowerCase().includes(q) ||
      e.srcIP.toLowerCase().includes(q)
    )
  }
  if (verdictFilter.value) {
    events = events.filter(e => e.verdict === verdictFilter.value)
  }
  return events
})

const verdictClass = (v: string) =>
  v === 'allow'
    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
    : 'bg-rose-500/10 text-rose-600 dark:text-rose-400'

async function fetchData() {
  await store.fetchAudit({
    keyword: searchQuery.value || undefined,
    verdict: verdictFilter.value || undefined,
  })
}

let debounce: ReturnType<typeof setTimeout>
watch([searchQuery, verdictFilter], () => {
  clearTimeout(debounce)
  debounce = setTimeout(fetchData, 300)
})

onMounted(fetchData)
</script>

<template>
  <div class="p-6 space-y-4">
    <!-- Filters -->
    <div class="flex flex-wrap items-center gap-3">
      <div class="relative max-w-xs">
        <Search class="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          v-model="searchQuery"
          :placeholder="t('sandbox.audit.searchPlaceholder')"
          class="h-9 pl-8 text-sm"
        />
      </div>
      <div class="flex gap-1.5">
        <Button
          v-for="opt in [
            { value: '', label: t('sandbox.audit.filterAll') },
            { value: 'allow', label: t('sandbox.audit.filterAllow') },
            { value: 'drop', label: t('sandbox.audit.filterDrop') },
          ]"
          :key="opt.value"
          :variant="verdictFilter === opt.value ? 'default' : 'outline'"
          size="sm"
          @click="verdictFilter = opt.value as '' | 'allow' | 'drop'"
        >
          {{ opt.label }}
        </Button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="store.auditLoading" class="flex gap-4">
      <div v-for="i in 4" :key="i" class="h-10 flex-1 animate-pulse rounded-lg bg-muted" />
    </div>

    <!-- Empty -->
    <div v-else-if="filteredEvents.length === 0" class="flex flex-col items-center gap-2 py-16 text-sm text-muted-foreground">
      <ShieldCheck class="size-10 opacity-40" />
      <p>{{ searchQuery || verdictFilter ? t('sandbox.audit.noMatch') : t('sandbox.audit.empty') }}</p>
    </div>

    <!-- Table -->
    <div v-else class="rounded-xl border border-border bg-card overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('sandbox.audit.colTime') }}</TableHead>
            <TableHead>{{ t('sandbox.audit.colSandbox') }}</TableHead>
            <TableHead>{{ t('sandbox.audit.colSrcIP') }}</TableHead>
            <TableHead>{{ t('sandbox.audit.colDst') }}</TableHead>
            <TableHead>{{ t('sandbox.audit.colProtocol') }}</TableHead>
            <TableHead>{{ t('sandbox.audit.colVerdict') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="event in filteredEvents" :key="event.id">
            <TableCell class="text-xs text-muted-foreground">
              {{ new Date(event.timestamp).toLocaleString() }}
            </TableCell>
            <TableCell class="font-medium text-sm">{{ event.sandboxName }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">{{ event.srcIP }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">
              {{ event.dstIP }}:{{ event.dstPort }}
            </TableCell>
            <TableCell class="text-xs uppercase text-muted-foreground">{{ event.protocol }}</TableCell>
            <TableCell>
              <Badge :class="verdictClass(event.verdict)" variant="secondary">
                {{ t(`sandbox.audit.${event.verdict}`) }}
              </Badge>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Commit**

```bash
git add fronted/src/pages/sandbox/audit.vue
git commit -s -m "feat(frontend): add agent traffic audit page"
```

---

### Task 11: Final Verification

- [ ] **Step 1: Run type check**

```bash
cd fronted && pnpm type-check 2>&1 | head -30
```

- [ ] **Step 2: Run tests**

```bash
cd fronted && pnpm test --run
```

- [ ] **Step 3: Run dev server and visually verify**

```bash
cd fronted && pnpm dev
```

Verify at `http://localhost:5173/`:
- Landing page renders all sections correctly
- Navigate to `/sandbox`, `/sandbox/tokens`, `/sandbox/audit` to verify new pages render
- Sidebar shows the new "Agent 沙箱" navigation group
- Dashboard shows 5 stat cards

- [ ] **Step 4: Commit any fixes**

```bash
git commit -s -m "chore(frontend): final verification fixes"
```
