---
title: AI Agent Security — Capability Verification
---

# AI Agent Security: What Lattice Actually Delivers

> 对照"AI Agent 零信任沙箱"这一商业定位，以实际代码验证每项能力的实现状态。
> 最后更新: 2026-05-18，基于 `cmd/lattice/cmd/sandbox/`、`internal/agent/gvisor/`、`api/v1alpha1/` 源码。

## 三大核心场景：代码级验证

### 场景 1：Agent 间"阅后即焚"加密私有网

> 主控系统通过 CRD 声明，一瞬间为两个 Agent 拉起专属 WireGuard 加密隧道。任务结束自动回收，不留痕迹。

| 能力点 | 状态 | 代码位置 |
|--------|------|---------|
| AgentIdentity CRD 声明式创建 | ✅ 已实现 | `api/v1alpha1/agent_identity_types.go` |
| 每个 Agent 独立 WireGuard keypair | ✅ 已实现 | `cmd/lattice/cmd/sandbox/sandbox_shared.go:28` — `sandboxCredentials{PrivateKey, JWT}` |
| Manager reconciler 自动下发 peer 配置 | ✅ 已实现 | `internal/server/controller/agent_identity_controller.go` |
| 网络隔离 (default-deny) | ✅ 已实现 | `LatticePolicy` CRD + iptables/eBPF enforcer |
| TTL 自动过期 | ⚠️ 部分实现 | Phase 切换到 `AgentPhaseExpired`，但 **不删除 CRD**（`agent_identity_controller.go:60-68`） |
| 任务结束网络资源回收 | ❌ 未实现 | 无 CRD 垃圾回收逻辑（GC） |

**诚实评估：核心路径 80% 打通。"阅后即焚"的"焚"差最后一步 GC。**

---

### 场景 2：外部 API 调用墙（Strict Egress Control）

> 对 Agent 实施极其严苛的白名单机制，精细到域名和 L7 层。未授权的 IP 直接在数据面默默丢包。

| 能力点 | 状态 | 代码位置 |
|--------|------|---------|
| CIDR 白名单 (`--egress-allow`) | ✅ PRO 已实现 | `sandbox_pro.go:92-105` — `net.ParseCIDR` |
| 默认 deny + Drop | ✅ PRO 已实现 | `sandbox_pro.go:73` — `--egress-default-deny` |
| HTTP CONNECT 正向代理 | ✅ PRO 已实现 | `sandbox_pro.go:279-353` — `httpForwardProxy` |
| ForwardListener（端口转发） | ✅ PRO 已实现 | `sandbox_pro.go:236` — `shimfwd.NewForwardListener` |
| Community 版 egress 控制 | ❌ 完全没有 | `sandbox_community.go:56-63` 只有 `--name/--server-url/--token` |
| **域名级过滤** | ❌ 未实现 | `PolicyAdapter.Allow` 签名只有 `dstIP + dstPort`（`shim_adapter.go:44`），没有 DNS 解析 |
| **L7 层过滤**（HTTP path/method/header） | ❌ 未实现 | 无相关代码 |

**诚实评估："精细到域名和 L7 层"目前不成立。只有 CIDR（IP 级）白名单。**

---

### 场景 3：多云/混合云 Agent 跨境协同

> 无缝编排跨云 K8s 节点，内地和海外 Agent 在同一个超轻量"虚拟大内网"里通信，干掉昂贵专线。

| 能力点 | 状态 | 代码位置 |
|--------|------|---------|
| WireGuard overlay 跨集群 | ✅ 已实现 | `cmd/lattice` + `cmd/manager` |
| ICE NAT 穿透（pion/ice v4） | ✅ 已实现 | `internal/agent/transport/` |
| LRP relay 兜底（QUIC） | ✅ 已实现 | `internal/agent/transport/` + WRRP/QUIC |
| Cluster Peering CRD | ✅ 已实现 | `api/v1alpha1/` |
| Network Peering | ✅ 已实现 | `features/network-peering` |
| 加密端到端通信 | ✅ 已实现 | WireGuard 内核/用户态实现 |

**诚实评估：场景 3 基本全部实现。这是 Lattice 最成熟的能力。**

---

## 完整能力矩阵（代码验证版）

| 能力 | Community | Pro | 验证文件 |
|------|-----------|-----|---------|
| WireGuard 加密隧道 | ✅ | ✅ | `cmd/lattice` |
| ICE NAT 穿透 (STUN/TURN) | ✅ | ✅ | `internal/agent/transport/` |
| LRP relay (TCP + QUIC) | ✅ | ✅ | `internal/agent/transport/` |
| K8s CRD Operator (kubebuilder) | ✅ | ✅ | `cmd/manager` |
| Dashboard UI | ✅ | ✅ | `fronted/` |
| gVisor 零权限沙箱 | ✅ | ✅ | `internal/agent/gvisor/` |
| 凭证持久化（重启恢复，mode 0600） | ✅ | ✅ | `sandbox_shared.go:42-65` |
| 本地审计日志 (JSONL) | ✅ | ✅ | `sandbox_community.go:67` |
| MCP Tool Tracing (`la_tool_spans`) | ✅ | ✅ | `models/tool_span.go` |
| Sub-agent Delegate API | ✅ | ✅ | `agent_isolation_router.go:47` |
| TTL 自动过期（设 phase=Expired） | ✅ | ✅ | `agent_identity_controller.go:60` |
| CIDR EgressFilter (`--egress-allow`) | ❌ | ✅ | `sandbox_pro.go:92-105` |
| ForwardListener (`--forward`) | ❌ | ✅ | `sandbox_pro.go:236` |
| HTTP CONNECT 代理 (`--proxy-addr`) | ❌ | ✅ | `sandbox_pro.go:279-353` |
| NATS 服务端流量审计 (`la_flow_events`) | ❌ | ✅ | `audit_consumer_pro.go` |
| Sandbox→NATS 审计推送 (`natsAuditWriter`) | ❌ | ❌ | 已规划，标注"待实现" |
| 域名级 egress 过滤 | ❌ | ❌ | 不存在 |
| L7 (HTTP) 过滤 | ❌ | ❌ | 不存在 |
| TTL 到期 CRD 自动清理 (GC) | ❌ | ❌ | 不存在 |

---

## 与竞品的差异化

| 维度 | Calico / Cilium | Lattice |
|------|----------------|---------|
| 核心定位 | K8s CNI 网络插件 | AI Agent 零信任网络沙箱 |
| 网络模型 | 扁平 / 三层路由 | WireGuard 加密 overlay mesh |
| Agent 隔离 | 靠 NetworkPolicy（IP/port） | gVisor 用户态沙箱 + WireGuard 身份 |
| Agent 身份 | Pod IP / ServiceAccount | WireGuard 公钥 + AgentIdentity CRD |
| 跨集群 | 需要额外配置（多集群 mesh） | 原生 Cluster Peering CRD |
| NAT 穿透 | 不适用 | ICE + LRP relay |
| 零权限部署 | 需要 privileged DaemonSet | `lattice sandbox start` 不需要 root |
| AI 框架集成 | 无 | Python SDK + MCP Server |
| TTL 自动过期 | 无 | AgentIdentity.ExpiresAt |
| 审计追踪 | 依赖外部系统 | MCP tool spans + 流量审计 |
| eBPF | Cilium 的强项 | PRO 有 eBPF TC enforcer，Community 用 iptables |

---

## 当前技术差距与下一步

1. **域名/L7 过滤** — gVisor netstack 内做 DNS 拦截 + 动态 IP 映射，工作量中等
2. **CRD GC** — TTL 到期后删除 AgentIdentity / LatticePeer，工作量小
3. **natsAuditWriter** — sandbox 侧将审计事件发布到 NATS，文档已标注"待实现"
4. **Community 版 egress 控制** — 至少给一个 `--egress-allow` 的 Community 简化版
