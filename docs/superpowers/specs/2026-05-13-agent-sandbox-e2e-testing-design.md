# Agent Sandbox E2E 测试设计

> 状态: 设计阶段 | 关联: `2026-05-11-agent-sandbox-and-ecosystem-design.md`

## 概述

为 AI Agent sandbox（`lattice-agent-sandbox` + gVisor netstack）增加端到端测试，覆盖 Agent 注册、沙箱启动、策略执行、审计和吊销全链路。

## 架构

### 测试拓扑

```
k3d 集群
├── latticed (All-in-One 控制面)
│
├── Pod: companion-agent
│   Container 1: lattice agent (标准, WG tunnel)
│   Container 2: nginx (:8080)
│   注册方式: lattice up --token
│   角色: 连通性目标
│   VPN IP: 10.100.0.10
│
└── Pod: lattice-agent-sandbox
    Container: lattice-agent-sandbox start --wg
    注册方式: POST /api/v1/agent-isolation/register
    角色: 被测沙箱
    VPN IP: 10.100.0.20
```

### 数据流

```
场景 allow:
  execInPod(sandbox) → curl 10.100.0.10:8080
    → gVisor netstack DialContext
    → wireguard-go encrypt → UDP :51820
    → companion agent → decrypt → nginx
    → 断言: 200 OK

场景 deny:
  execInPod(sandbox) → curl 1.2.3.4:80
    → gVisor netstack DialContext
    → wireguard-go 查不到 peer → 丢包/timeout
    → 断言: 连接失败
```

## 前提条件

### --wg flag 实现

补齐 `--wg` flag 功能，将 wireguard-go 附着到 gVisor netstack。

**新增** `internal/agent/gvisor/wg_bridge.go` (`//go:build pro`)：
- 创建 wireguard-go UDP bind
- 桥接 gVisor netstack ↔ WireGuard:
  - 出站: `DialContext()` → wireguard-go encrypt → UDP bind → :51820 → Lattice overlay
  - 入站: UDP bind :51820 → decrypt → netstack deliver
- 复用 `sandbox.go` 中已有的 `pumpOutbound` 和 `Outbound` 回调

**修改** `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go`：
- `--wg` 启用时创建 WireGuardBind 传入 `gvisor.New(Config{WireGuardBind: bind})`

### 审计输出

`start_sandbox_pro.go` 的 AuditWriter 写本地 JSONL 文件 `/tmp/lattice-audit.jsonl`：

```jsonl
{"pid":12345,"dst_ip":"10.100.0.10","dst_port":8080,"protocol":"tcp","verdict":"allow"}
{"pid":12345,"dst_ip":"1.2.3.4","dst_port":80,"protocol":"tcp","verdict":"drop"}
```

e2e 测试通过 `execInPod cat /tmp/lattice-audit.jsonl` 读取并断言。

## 测试文件

### 新增 `test/e2e/agent_sandbox_test.go`

沿用现有 Ginkgo `Ordered` 模式，6 个场景。

**BeforeAll:**
1. `login(manageUrl)`
2. `createWorkspace("wf-e2e-sandbox-<ts>")`
3. `generateJoinToken(wsID)` ← 给 companion 用
4. `createEnrollmentToken(wsID, allowedTools)` ← 给 sandbox 用
5. `hostAliasesForNATS()`
6. `deployCompanionPod()` ← 标准 lattice agent + nginx
7. `waitForReady(companion)` → companion VPN IP
8. `deploySandboxPod()` ← lattice-agent-sandbox --wg
9. `waitForWGIP(sandbox)` → sandbox VPN IP
10. 创建 allow-all Policy

**测试场景:**

| # | It | 断言 |
|---|-----|------|
| 1 | `agent registration creates AgentIdentity` | AgentIdentity CRD exists, Phase=Active |
| 2 | `sandbox connects to companion via overlay` | `curl companion-ip:8080` → 200 OK |
| 3 | `sandbox traffic to non-lattice IP is blocked` | `curl 1.2.3.4:80` → timeout/non-200 |
| 4 | `policy deny blocks specific destination` | 更新 Policy 为 deny companion → `curl` timeout |
| 5 | `audit events captured` | `cat /tmp/lattice-audit.jsonl` 包含 allow/drop 事件 |
| 6 | `agent revoke stops sandbox connections` | DELETE /agents/:name → Phase=Revoked → 新连接失败 |

**AfterAll:** `cleanupWorkspace` + 清理 AgentIdentity + sandbox Pod

### 修改 `test/e2e/helpers_test.go`

新增辅助函数:
- `createEnrollmentToken(wsID, tools)` → token
- `registerSandboxAgent(serverURL, token, name, pubkey)` → JWT + VPN IP
- `deploySandboxPod(token, image)` → pod
- `revokeAgent(serverURL, name, namespace)`
- `readAuditLog(pod)` → []AuditEvent
- `execInSandbox(pod, cmd)` → output

## 实现范围

### 必须实现

| 变更 | 位置 |
|------|------|
| --wg flag 实现 | `internal/agent/gvisor/wg_bridge.go` (新) |
| --wg flag 启用 | `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` |
| Audit JSONL 输出 | `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` |
| e2e 测试文件 | `test/e2e/agent_sandbox_test.go` (新) |
| e2e 辅助函数 | `test/e2e/helpers_test.go` |
| PRO e2e sandbox 镜像 | CI / k3s Dockerfile |

### 不改

| 项 | 原因 |
|----|------|
| runsc / gVisor OCI runtime | gVisor netstack 编译进二进制，不需要 runsc |
| k3d 集群镜像 | 不需要预装额外工具 |
| 控制面审计 API / DB | 审计走本地文件 |
| PolicyChecker | 保持 nil（全放行），deny 靠"非 Lattice IP 无路由" |

## Sandbox Pod Spec

```yaml
containers:
- name: lattice-agent-sandbox
  image: ghcr.io/xxx/lattice-agent-sandbox:pro-xxx
  args: [start, --name=e2e-sandbox, --server-url=http://latticed:8080, --token=<token>, --wg]
  ports:
  - containerPort: 51820
    protocol: UDP
  # 不需要: privileged, CAP_NET_ADMIN, /lib/modules, --local-ip
```

companion Pod 复用现有 `deployAgentDeployment` 模式，加一个 nginx 容器在 8080。

## CI 集成

独立 workflow `sandbox-e2e.yml`，仅 PRO 构建时触发：

- **触发**: PR 带 `run-pro` label
- **构建**: `lattice-agent-sandbox` PRO 镜像 + `latticed` PRO 镜像 + companion agent PRO 镜像
- **集群**: k3d，和现有 e2e 使用相同模式
- **运行**: `go test ./test/e2e/... -run AgentSandbox`
- **独立**: 不依赖现有 e2e workflow，sandbox 测试不影响基础 e2e

---

*最后更新: 2026-05-13*
