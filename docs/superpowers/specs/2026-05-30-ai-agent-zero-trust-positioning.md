# Lattice 安全定位：AI Agent Zero Trust

**日期**：2026-05-30
**状态**：已确认
**背景**：基于与 Google SAIF 框架的对比分析和产品战略讨论

---

## 一、结论先行

**Lattice 的安全定位是：AI Agent Zero Trust。**

不是"AI agent 安全平台"，不是"SAIF 合规工具"，而是专门解决 AI agent 通信层的 Zero Trust 问题——这个领域目前没有人做好，Lattice 有真实的差异化。

---

## 二、为什么不做全栈安全

### Google SAIF 的六个层次

SAIF 框架涵盖 AI 系统安全的全部层次：供应链安全、运行时进程隔离、网络隔离、访问控制、可观测性、事件响应。

### Lattice 在安全栈的位置

```
L5  供应链安全（模型完整性、工具签名、SBOM）
    → 不是 Lattice 的事。Sigstore、SBOM 工具链负责。

L4  运行时异常检测（行为基线、anomaly detection、威胁情报）
    → 不是 Lattice 的事。Falco、Datadog、Sysdig 负责。

L3  进程隔离（syscall 过滤、overlayfs、沙箱运行时）
    → 不是 Lattice 的核心。薄集成即可（见第四节）。

────────────────────────────────────────────────────
L2  工具级 Zero Trust（AgentPolicy、MCP 审计、工具调用控制）
    → Lattice 的独特价值。没有任何工具专门做这一层。

L1  网络级 Zero Trust（WireGuard、PeerIdentity、LatticePolicy）
    → Lattice 的根基。已有强实现。
────────────────────────────────────────────────────
```

**L3-L5 有成熟竞品，Lattice 没有护城河。L1+L2 是空白市场，Lattice 有先发优势。**

---

## 三、AI Agent Zero Trust 的五个支柱

### 3.1 身份（Identity）

每个 agent 有密码学身份，跨环境可追踪，不依赖 IP 地址或主机名。身份分三层：WireGuard 公钥（传输层）→ LatticePeer（设备注册）→ PeerIdentity（逻辑角色），策略绑定在最顶层的逻辑角色上，设备迁移和密钥轮换不影响策略有效性。

技术细节见 [PeerIdentity Zero Trust 设计](./2026-05-29-peer-identity-zero-trust-design.md)。

**与现有方案的差异**：Istio mTLS 面向微服务（代码确定、行为可预期）。AI agent 的行为由 LLM 决定，不可预期，需要更强的运行时控制，不只是证书认证。

### 3.2 策略（Policy）

两层策略，关注点分离：

**网络层（LatticePolicy）**：
- agent 能不能到达某个 overlay IP/CIDR
- 基于 PeerIdentity，不基于 IP（Zero Trust 核心原则）
- 执行点：iptables/eBPF，agent 无法绕过

**工具调用层（AgentPolicy）**：
- agent 能不能调某个 MCP server 的某个工具
- 粒度：`worker-agent → file-server → read_file` ✅，`exec_command` ❌
- 执行点：MCP proxy，HTTP 层拦截 JSON-RPC

两层都通过才放行。这是目前任何工具都没有做到的粒度。

### 3.3 加密（Encryption）

所有 agent↔agent、agent↔MCP server（内部模式）通信经过 WireGuard 加密。

- P2P 加密，不经过中心节点
- 跨云/跨集群透明
- WireGuard 密钥周期轮换，不影响上层 PeerIdentity 和 Policy

### 3.4 审计（Audit）

语义级审计，不是网络事件日志。

```json
{
  "agentName": "worker-1",
  "mcpServer": "file-tools",
  "tool": "read_file",
  "params": {"path": "/data/report.pdf"},
  "verdict": "allow",
  "latencyMs": 23
}
```

记录的是"谁调了什么工具、传了什么参数、结果怎样"，不是"TCP connect to 10.0.7.5:3000"。

### 3.5 吊销（Revocation）

失控 agent 的即时处置分两级：

- **软吊销**：删除或修改 AgentPolicy，tool call 立即被拒绝（PolicyCache 15s 内失效）
- **硬吊销**：删除 PeerIdentity，密码学级别隔离，所有 peer 拒绝握手。详见 [PeerIdentity 立即撤销流程](./2026-05-29-peer-identity-zero-trust-design.md#场景-c立即撤销)。

---

## 四、Sandbox 策略：网络层是 Lattice 的边界

### 4.1 `lattice sandbox run` 的定位

`lattice sandbox run` 继续存在，但职责边界清晰：

**Lattice 负责（网络层隔离）**：
- 创建隔离的 Linux network namespace（agent 只能看到 wg0，看不到宿主网络）
- veth pair 连接 agent netns 与宿主 wg0
- agent netns 内 iptables REDIRECT → MCP proxy（透明强制，agent 无法绕过）
- 出口策略（egress CIDR 白名单）

**Lattice 不负责（进程层隔离）**：
- syscall 过滤
- 文件系统隔离（overlayfs/read-only rootfs）
- 进程树隔离

### 4.2 为什么不自己做进程隔离

**gVisor 的问题**：
- 必须预装 runsc 二进制（用户无法零配置使用）
- Docker 需要 `--privileged`（多数企业安全策略禁止）
- K8s 需要集群管理员配置 runtimeClass（非普通用户权限）
- **关键**：gVisor 提供的是 syscall 级拦截，不提供 MCP 工具调用可见性——MCP 审计仍然需要 HTTP proxy，gVisor 不解决这个问题
- gVisor 拦截的是 `connect(fd, 10.0.7.5:3000)` 和原始字节流，不是 `tool=read_file, path=/etc/passwd`

**自己写 namespace 进程隔离的问题**：
- 进入 Falco/Aqua/Sysdig 的地盘，没有竞争优势
- mntns（只读 rootfs）在 Docker 内有 overlayfs-on-overlayfs 兼容性问题
- 高维护负担，与核心竞争力无关

### 4.3 进程隔离的集成方式

Lattice 在文档和 UI 中说明：

> 如需进程级隔离，在 K8s 中配合 gVisor runtimeClass 使用：
> ```yaml
> spec:
>   runtimeClassName: gvisor
> ```
> Lattice 的 network namespace 隔离与 gVisor 进程隔离正交，可以叠加使用。

这样 Lattice 保持在自己的核心能力范围内，同时给高安全需求用户提供清晰的升级路径。

---

## 五、与 SAIF 的诚实对齐声明

Lattice 不声称覆盖全部 SAIF，而是清晰地说明覆盖范围：

| SAIF 原则 | Lattice 覆盖 | 说明 |
|---|---|---|
| 身份验证（持续验证） | ✅ | WireGuard 密码学身份 + PeerIdentity |
| 最小权限访问 | ✅ | LatticePolicy（网络层）+ AgentPolicy（工具层）|
| 加密传输 | ✅ | WireGuard E2E 加密 |
| 可审计性 | ✅（工具调用层）| MCP 审计日志；syscall 审计不覆盖 |
| 即时响应/吊销 | ✅ | PeerIdentity 删除 → 密码学级别隔离 |
| 进程隔离 | ⚠️ 集成 | 文档指向 gVisor/Kata，Lattice 不实现 |
| 供应链安全 | ❌ 不覆盖 | 超出范围 |
| 运行时异常检测 | ❌ 不覆盖 | 超出范围（Phase N） |

**核心主张**：Lattice 是目前唯一在 **网络层 + 工具调用层** 同时实现 Zero Trust 的工具，这两层是 AI agent 安全的关键路径，也是现有工具（Istio、Kong、LangSmith）都没有覆盖的空白。

---

## 六、竞品边界

| 工具 | 定位 | 与 Lattice 的关系 |
|---|---|---|
| Istio / Tetrate | 微服务 mTLS（行为可预期） | 不冲突，Lattice 面向 AI agent（行为不可预期） |
| Falco / Sysdig | 运行时 syscall 审计 | 互补，Lattice 做网络+工具层，Falco 做进程层 |
| LangSmith / Helicone | LLM observability | 只观测，无网络隔离和策略执行 |
| Kong / APISIX | API Gateway（ingress） | 不同层，Lattice 做 agent↔agent 通信 |
| **Lattice** | **AI agent 通信的 Zero Trust** | 本文档描述的定位 |

---

## 七、实现路线图（安全方向）

### 已完成
- WireGuard overlay（网络加密）
- LatticePolicy（网络级访问控制）
- MCPServer CRD + AgentPolicy CRD（工具级访问控制）
- MCP proxy（工具调用拦截和审计）

### 进行中
- PeerIdentity CRD（身份层，与网络拓扑解耦）

### 下一步
- **Sandbox netns 隔离**：用 Linux network namespace 替换 gVisor CustomTUN，恢复透明网络强制执行（agent 无法绕过 MCP proxy）
- **审计 API**：结构化查询 MCP 调用历史
- **告警**：异常调用模式检测（频率突变、高风险工具调用）

### 不做（明确边界）
- syscall 级进程隔离（留给 gVisor/Kata）
- 供应链安全（留给 Sigstore/SBOM）
- 模型行为分析（留给 LLM observability 工具）
