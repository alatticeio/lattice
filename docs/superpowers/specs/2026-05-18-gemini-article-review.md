# Gemini 推文评审 & 终稿

> Gemini 初稿 → 代码现状对齐评审 → 终稿优化版本
> 基准：`docs/competitiveness.md`（代码级能力验证）+ `api/v1alpha1/` CRD 实际定义。

---

# Part 1：评审

## 一、总体评价

| 维度 | 评分 | 说明 |
|------|------|------|
| 叙事吸引力 | ★★★★☆ | 威胁模型切入有力，"三个独特行为"是最强段落 |
| 技术准确性 | ★★☆☆☆ | CRD 名称错误、能力夸大、架构描述与现状脱节 |
| 差异化表达 | ★★★☆☆ | 部分比较路径成立，部分不成立 |
| 行动号召 | ★★★☆☆ | GitHub 占位符、CTA 偏弱 |

**核心问题：文章描述的是一款"理想中的产品"，而非"当前代码能跑出来的产品"。** 在开源社区语境下，技术准确性是第一位的。

## 二、逐段评审

### 2.1 开篇：传统 CNI + NetworkPolicy 的局限 ✅

没问题。Calico/Cilium + NetworkPolicy 确实是当前标准方案。背景铺垫准确。

### 2.2 三个 AI Agent 独特行为 ✅✅✅ —— 全文最佳段落

| 行为 | 真实性 |
|------|--------|
| Prompt Injection → 横向移动 | ✅ 真实威胁，整个 agent-isolation 设计的核心威胁模型 |
| Dynamic Mesh-on-Demand（阅后即焚） | ✅ 真实需求，multi-agent 协作的核心场景 |
| Data Exfiltration via Hallucination | ✅ 真实威胁，传统防火墙无法区分真实 API 调用和幻觉外泄 |

### 2.3 BYOC + SaaS Control Plane ⚠️ 偏离现状

| 文案 | 代码现状 | 偏差 |
|------|----------|------|
| "SaaS Control Plane" | `latticed` 是自托管一体化控制面 | 没有任何 SaaS 托管版 |
| "Policies synced via gRPC/HTTPS from SaaS" | 用户 `kubectl apply` CRD → Manager reconciler → NATS → Agent | 不存在外部 SaaS 同步 |
| "BYOC" | 纯自托管 | BYOC 暗示存在托管版，实际没有 |

### 2.4 CRD YAML 示例 ❌ 致命错误

| Gemini 写的 | 实际代码 |
|-------------|----------|
| `kind: AgentNetworkSandbox` | `kind: AgentIdentity` |
| `apiVersion: networking.lattice.ai/v1alpha1` | `apiVersion: lattice.alattice.io/v1alpha1` |
| `spec.internalIsolation.blockAllClusterInternal` | 不存在 |
| `spec.egressRules.allowedDomains` | 不存在，只有 CIDR (`ipBlock.cidr`) |
| `spec.auditing.enabled` / `logLevel` | 实际是 `auditLevel: none/write/full` |

### 2.5 Egress Filtering ❌ 严重不准确

- 域名级过滤 **不存在**——`PolicyAdapter.Allow` 签名只有 `dstIP + dstPort`
- L7 过滤 **不存在**
- "api.openai.com or github.com" 这种域名白名单当前做不到，只有 CIDR

### 2.6 "Zero Performance Overhead" ❌ 不诚实

gVisor 用户态 netstack + WireGuard-Go 有性能开销，不是 "zero overhead"。

### 2.7 其他小问题

- GitHub URL 是占位符 `github.com/your-org/lattice`
- 缺少 Community vs Pro 功能边界区分
- 中英文混杂（"阅后即焚"在英文文章里不翻译）

---

# Part 2：终稿

> 基于 Gemini 初稿改写。修正了所有与代码现状不符的表述，保留原文的威胁模型驱动叙事。
> CRD 字段全部来自 `api/v1alpha1/` 实际类型定义。

---

In a typical production Kubernetes cluster, we rely on CNI plugins (Calico, Cilium) and static NetworkPolicies defined by label selectors to isolate microservices. This works perfectly when Microservice A only ever needs to talk to Microservice B over a predictable, hardcoded gRPC route.

**AI Agents break this model entirely** due to three unique behaviors:

1. **Prompt Injection → Insider Threat**: If a coding agent suffers a prompt injection attack, it can execute malicious commands inside its own runtime container. It suddenly becomes an insider threat — running port scans across your internal subnets, probing your databases, or scraping telemetry endpoints, all from within your cluster.

2. **Dynamic Mesh-on-Demand**: Multi-agent choreography requires short-lived, dynamic communication topologies. Agent A spawns Agent B and Agent C to solve a reasoning task — they need encrypted P2P communication for 45 seconds, then vanish. Static YAML-based NetworkPolicies cannot keep up with this sub-minute lifecycle.

3. **Data Exfiltration via Hallucination**: An agent processing proprietary codebases or PII can be tricked into uploading sensitive context to an unauthorized external IP, disguised as a legitimate "API call." Traditional firewalls can't distinguish between a real API request and a hallucinated exfiltration attempt.

---

## Enter Lattice

Lattice is a **self-hosted, Kubernetes-native overlay network** built on WireGuard and eBPF. It treats AI agent network security not as an infrastructure afterthought, but as a declarative, application-level primitive — something you define alongside the agent itself.

### Architecture: Everything Stays Inside Your Boundary

Security teams will never trust a third-party SaaS that routes sensitive AI traffic or LLM tokens through an external cloud. Lattice takes the opposite approach:

- **Control plane** (`latticed`) runs inside your cluster — NATS signaling, SQLite/MySQL, REST API, K8s operator
- **Data plane** (`lattice` agent) runs as a DaemonSet or standalone binary — WireGuard tunnels, ICE NAT traversal, eBPF policy enforcement
- **Agent sandbox** (`lattice sandbox start`) runs in user-space via gVisor netstack — zero privileges, no kernel modules required

Your tokens, your agent memory, and your network traffic never leave your cluster boundary.

### How the Sandbox Works

**Step 1 — Give every agent an identity, not just a Pod IP:**

```yaml
apiVersion: lattice.alattice.io/v1alpha1
kind: AgentIdentity
metadata:
  name: claude-coder-sandbox
  namespace: ai-agents
spec:
  peerRef: claude-coder-peer          # 绑定的 WireGuard 节点
  allowedTools:                        # 工具白名单
    - "read_file"
    - "write_file"
    - "search_code"
  allowedNamespaces:                   # 限制可操作的 K8s 命名空间
    - "ai-agents"
  sandbox: gvisor                      # gVisor 用户态沙箱隔离
  auditLevel: full                     # 审计：记录所有 tool call
  enforcementMode: enforce             # 违规处理：直接阻断
  expiresAt: "2026-05-19T00:00:00Z"   # 阅后即焚：任务结束自动过期
  spawnableRoles:                      # 允许派生的子 Agent 角色
    - "code-reviewer"
```

**Step 2 — Define what the agent can reach:**

```yaml
apiVersion: lattice.alattice.io/v1alpha1
kind: LatticePolicy
metadata:
  name: coder-egress
  namespace: ai-agents
spec:
  network: ai-sandbox-net
  peerSelector:
    matchLabels:
      agent-role: code-executor
  egress:
    - to:
        - cidr: 10.0.0.0/8           # 内部 LLM Gateway
      ports:
        - protocol: TCP
          port: 8080
    - to:
        - cidr: 160.79.0.0/16        # api.anthropic.com IP 段
      ports:
        - protocol: TCP
          port: 443
  action: ALLOW                       # 默认拒绝，仅放行白名单
  expiresAt: "2026-05-19T00:00:00Z"  # 任务结束后策略自动清理
```

**The result:** even if the code inside `claude-coder-sandbox` is fully compromised via a malicious prompt, the attacker cannot pivot to your internal databases or leak data to a rogue C2 server. The data plane drops unauthorized packets silently at the source.

### On-Demand Encrypted Mesh

When Agent A spawns Agent B for a collaborative reasoning loop, Lattice's Manager reconciler detects the `AgentIdentity` creation and provisions a peer-to-peer WireGuard tunnel automatically. Both agents get unique keypairs and IPs from the built-in IPAM. The tunnel is invisible to other workloads. When `expiresAt` fires, the identity transitions to `Expired` — connection torn down, keys invalidated. **"Read-and-burn" networking.**

### Egress Filtering (Pro)

The Pro edition adds CIDR-level egress filtering inside the gVisor netstack. Default-deny with explicit allowlists — agents can only connect to IP ranges you declare. Unauthorized packets are dropped before they reach the host network.

> Domain-level (L7) filtering is in active development. Today we enforce at the IP level because DNS interception in a user-space netstack requires careful TTL and caching semantics — we'd rather ship correct CIDR filtering today than broken domain filtering tomorrow.

---

## Open-Core Model

| Capability | Community | Pro |
|------------|-----------|-----|
| gVisor zero-privilege sandbox | ✅ | ✅ |
| WireGuard mesh + ICE NAT traversal | ✅ | ✅ |
| AgentIdentity CRD + lifecycle | ✅ | ✅ |
| MCP tool tracing (`la_tool_spans`) | ✅ | ✅ |
| Sub-agent delegation API | ✅ | ✅ |
| Credential persistence (restart-safe) | ✅ | ✅ |
| LatticePolicy (label-based ACLs) | ✅ (iptables) | ✅ (eBPF TC) |
| CIDR egress filter (`--egress-allow`) | — | ✅ |
| HTTP forward proxy | — | ✅ |
| NATS flow audit | — | ✅ |
| Time-travel debugging | — | ✅ |

---

## Why This Matters for AI Platform Teams

Building a production AI platform isn't just about connecting to LLM APIs — it requires a **security boundary around autonomous entities that think and act on their own**.

- **No privileged containers**: `lattice sandbox start` runs with zero kernel privileges. No root, no `CAP_NET_ADMIN`.
- **Self-hosted data sovereignty**: Everything stays inside your VPC. No third-party cloud sees your tokens or traffic.
- **K8s-native**: Everything is a CRD. GitOps-friendly. No external service dependency.
- **Tool-call audit trail**: Every agent tool invocation is traced with caller identity, timestamps, and parameters — queryable via REST API.

---

**GitHub**: [github.com/alatticeio/lattice](https://github.com/alatticeio/lattice)
**Docs**: [docs.alattice.io](https://docs.alattice.io)

*Lattice is built by platform engineers, for platform engineers. The core sandboxing engine is open-source (Apache 2.0). Domain-level egress filtering, TTL-based CRD GC, and eBPF cgroup-based PID-to-TUN binding are in active development — feedback and contributions welcome.*
