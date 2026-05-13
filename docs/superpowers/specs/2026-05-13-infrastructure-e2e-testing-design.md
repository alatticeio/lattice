# Lattice 基础设施 E2E 测试设计

> 状态: 设计阶段 | 关联: `2026-05-13-agent-sandbox-e2e-testing-design.md`

## 概述

为 Lattice 基础设施层（latticed 控制面 + 标准 lattice agent + CRD 控制器）提供端到端测试覆盖。涵盖 agent 注册、WireGuard 隧道连通、多网络隔离、策略执行、重启恢复以及 AI API 可用性。

独立的 Agent Sandbox e2e 测试见 `2026-05-13-agent-sandbox-e2e-testing-design.md`。

---

## 测试架构

### 框架选型

- **Ginkgo v2 + Gomega**：BDD 风格，`Ordered` 容器保证步骤串行
- **k3d**：单节点 k3s 集群（no LB, no traefik），`lattice-system` namespace 运行 All-in-One 控制面
- **SPDY executor**：`execInPod` 通过 K8s API 在 Pod 内执行命令（`wg show`, `iptables -L`, `ping`, `wget`）
- **controller-runtime client**：直连 CRD（LatticePeer, LatticeNetwork, LatticePolicy），验证控制器行为

### 测试拓扑

```
k3d 集群
├── lattice-system (ns)
│   ├── latticed Deployment (All-in-One 控制面)
│   ├── lattice-nats-service (NATS)
│   └── lattice-api-service (HTTP :8080, ClusterIP)
│
└── wf-<workspace-id> (ns, 每个 Spec 独立)
    ├── Deployment: agent-A (lattice up --token)
    ├── Deployment: agent-B (lattice up --token)
    ├── CRD: LatticePeer (A, B)
    ├── CRD: LatticeNetwork
    └── CRD: LatticePolicy (ALLOW / DENY)
```

### Suite 生命周期

```
BeforeSuite (全局一次):
  ├─ 加载 kubeconfig
  ├─ 创建 kubernetes.Clientset + controller-runtime Client
  ├─ 注册 Lattice CRD Scheme
  ├─ 登录 (admin / 123456 → JWT)
  └─ 写入 platform settings (NATS URL)

Per Spec (每个 Ordered 容器):
  ├─ BeforeAll: login → createWorkspace → generateJoinToken
  │            → hostAliasesForNATS → deployPods → waitReady
  │            → getNetwork → createAllowAllPolicy
  ├─ It × N: 测试场景
  └─ AfterAll: cleanupWorkspace (CRD 去 finalizer + 删除 + ns 删除)

After Suite (ReportAfterSuite):
  └─ All passed → delete ns
     Any failed  → preserve for investigation
```

---

## 公共辅助函数

所有函数位于 `test/e2e/helpers_test.go`，使用 Ginkgo `By()` 和 `Expect()` 断言。

### HTTP 层

| 函数 | 签名 | 功能 |
|------|------|------|
| `apiPOST` | `(url, token string, body any) → (int, *resp.Response)` | 带 Bearer auth 的 JSON POST |
| `login` | `(manageURL string) → string` | admin 登录，返回 JWT |
| `createWorkspace` | `(manageURL, token, nsName string) → string` | 创建 workspace，返回 ID |
| `generateJoinToken` | `(manageURL, token, workspaceID string) → string` | 生成 agent 加入令牌 |

### K8s 部署层

| 函数 | 功能 |
|------|------|
| `hostAliasesForNATS` | 发现 NATS ClusterIP → 构造 `signaling.alattice.io` hostAlias |
| `deployAgentDeployment` | 创建 Deployment（privileged, CAP_NET_ADMIN, /lib/modules 挂载, `lattice up --token`） |
| `waitForPodRunningReady` | `Eventually` 等待 Pod Running + 所有容器 Ready |
| `waitForWGIP` | `Eventually` 等待 LatticePeer CRD `AllocatedAddress` 赋值 |
| `getNetworkName` | 从 LatticePeer 提取网络名 |

### 策略层

| 函数 | 功能 |
|------|------|
| `createAllowAllPolicy` | 创建 ALLOW LatticePolicy（同网络 peer 互访） |

### 连通性断言层

| 函数 | 功能 |
|------|------|
| `pingWithRetry` | `Eventually` ping，断言 0% packet loss |
| `assertPingBlocked` | `Eventually` ping，断言非 0% loss 或 command failed |
| `execInPod` | SPDY 远程执行命令，返回 stdout |

### 清理层

| 函数 | 功能 |
|------|------|
| `cleanupWorkspace` | 移除 LatticePolicy/Peer/Network CRD finalizer → 删除 CRD → 删除 namespace（best-effort） |

---

## 测试场景

### Spec 1: Core Connectivity

**文件**: `e2e_test.go`
**容器**: `Describe("Lattice Core Connectivity E2E", Ordered)`

全链路验证：登录 → 创建 workspace → 生成 token → 部署双 Pod → 建立 WireGuard 隧道 → 双向 ping。

| # | It | 断言 |
|---|-----|------|
| 1 | Full chain: Login → Create Workspace → Generate Token → Deploy Pod → Verify tunnel | `login` 返回 token；`createWorkspace` 返回 workspace ID；`generateJoinToken` 返回 join token；`deployAgentDeployment` 创建成功；`waitForPodRunningReady` 两 Pod Ready；`waitForWGIP` 两 Peer 获分配 IP；`createAllowAllPolicy` 创建成功；`pingWithRetry` A→B 和 B→A 均 0% loss |

---

### Spec 2: Multi-Network Isolation

**文件**: `multi_network_test.go`
**容器**: `Describe("Multi-Network Isolation Test", Ordered)`
**拓扑**: 3 个 agent（mnet-a, mnet-b, mnet-c），默认网络 + 新建 network 2

| # | It | 断言 |
|---|-----|------|
| 1 | Switch mnet-c to network 2, verify controller re-allocates IP | `LatticePeer.Spec.Network` 更新后 `ActiveNetwork` 切换为 net2，新 IP 已分配 |
| 2 | Same network connectivity: mnet-a → mnet-b ping should succeed | `pingWithRetry` A→B 0% loss（同网络） |
| 3 | Cross-network isolation: Pods in different networks should not communicate | `assertPingBlocked` A→C 阻断 |
| 4 | Cross-network isolation: mnet-b → mnet-c should also be blocked | `assertPingBlocked` B→C 阻断 |

---

### Spec 3: Policy CRUD Lifecycle

**文件**: `policy_changes_test.go`
**容器**: `Describe("Policy CRUD Lifecycle", Ordered)`
**拓扑**: 1 个 agent（pc-peer）

| # | It | 断言 |
|---|-----|------|
| 1 | Create an ALLOW policy and verify CRD creation succeeds | `LatticePolicy` 创建成功，`Spec.Action=ALLOW`，`Spec.Network` 匹配 |
| 2 | List all policies under the workspace, should include the newly created policy | `LatticePolicyList` 包含 `e2e-pc-allow` |
| 3 | Create a second DENY policy and verify both policies coexist | `LatticePolicy` DENY 创建成功；`execInPod iptables -L LATTICE-EGRESS` 链存在 |
| 4 | After deleting the first policy, the second policy should still be effective | 删除后 `NotFound`；DENY policy 仍存在；iptables 链仍存在 |

---

### Spec 4: Peer Restart Resilience

**文件**: `peer_restart_test.go`
**容器**: `Describe("Multi-Peer Restart Resilience", Ordered)`
**拓扑**: 2 个 agent（restart-a, restart-b）

| # | It | 断言 |
|---|-----|------|
| 1 | Baseline: Connectivity between two peers is normal | A→B 和 B→A `pingWithRetry` 双向 0% loss |
| 2 | Restart restart-a: Pod deleted and recreated, tunnel should recover | Delete Pod → `waitForPodRunningReady` 新 Pod Ready → `waitForWGIP` IP 确认 → `wg show` 接口重建 → `pingWithRetry` A→B 恢复 |
| 3 | Restart both Peers simultaneously: tunnel recovers after both rebuild | 同时删除 A+B → 等两 Pod Ready → `wg show` 两接口重建 → 双向 `pingWithRetry` 均恢复 |

---

### Spec 5: Agent Resilience

**文件**: `resilience_test.go`
**容器**: `Describe("Agent Restart Resilience Test", Ordered)`
**拓扑**: 1 个 agent（res-pod）

| # | It | 断言 |
|---|-----|------|
| 1 | Baseline: Create ALLOW policy and verify tunnel status | LatticePeer 获取 network → `createAllowAllPolicy`；`wg show` 接口存在 |
| 2 | Agent process crash recovery: tunnel should be automatically rebuilt | Delete Pod → 新 Pod Ready → `waitForWGIP`IP 确认 → `wg show` 接口重建 → `ip addr show wf0` IP 与原 IP 一致 → `iptables -L LATTICE-EGRESS` 包含 ACCEPT 规则 |

---

### Spec 6: AI Debug & Intent Engine

**文件**: `ai_debug_intent_test.go`
**容器**: `Describe("AI Debug & Intent Engine", Ordered)`
**拓扑**: 仅 workspace（无需 Pod）

| # | It | 断言 |
|---|-----|------|
| 1 | Snapshot API: 列出工作空间的快照列表（GET） | `GET /api/v1/workspaces/{id}/snapshots` 返回 200 |
| 2 | AI Debug API: 发送调试请求 | `POST /api/v1/ai/debug` 返回 200（AI 已配置时） |
| 3 | AI Intent API: 提交网络意图计划 | `POST /api/v1/ai/intent/plan` 返回 200（AI 已配置时） |
| 4 | AI Tools API: 列出可用工具 | `POST /api/v1/ai/tools` 返回 200（AI 已配置时） |

---

## 待补充场景

以下场景已识别但尚未实现：

### Spec 7: 策略流量强制执行（计划中）

**目标**: 验证 DENY 策略实际阻断数据面流量（非仅 iptables 链存在性检查）。

| # | It | 断言 |
|---|-----|------|
| 1 | ALLOW 策略下 A→B 连通 | `pingWithRetry` A→B 0% loss |
| 2 | 切换到 DENY 策略后 A→B 阻断 | `assertPingBlocked` A→B 阻断 |
| 3 | 切回 ALLOW 策略后 A→B 恢复 | `pingWithRetry` A→B 0% loss |

**依赖**: Spec 3（Policy CRUD）已验证策略 CRUD 操作，本 spec 验证其**数据面效果**。

---

### Spec 8: Token 生命周期（计划中）

**目标**: 验证 join token 过期/撤销后的 agent 行为。

| # | It | 断言 |
|---|-----|------|
| 1 | 正常 token agent 可加入 | agent 注册成功，LatticePeer Active |
| 2 | 过期 token agent 被拒绝 | 注册失败，错误信息明确 |
| 3 | 已撤销 token agent 被拒绝 | 注册失败 |
| 4 | 运行中 agent 在 token 过期后保持连接 | 已有隧道不受影响 |

**依赖**: 需要 API 支持 token TTL 设置和 token 撤销接口。

---

### Spec 9: Relay 中继回退（计划中）

**目标**: 验证 ICE P2P 直连不可用时 LRP relay 接管。

| # | It | 断言 |
|---|-----|------|
| 1 | P2P 直连正常时走 ICE | 流量经 P2P 通道，延迟较低 |
| 2 | 阻断 P2P 路径后流量切换到 relay | `assertPingBlocked` 短暂丢失后恢复（relay 接管） |
| 3 | P2P 路径恢复后切回直连 | 流量回到 P2P 通道 |

**依赖**: k3d 中模拟网络分区（network policy 或 iptables 阻断 UDP 直连端口）。

---

## 运行方式

### 本地

```bash
# 启动 k3d 集群
make e2e-setup

# 部署 latticed All-in-One + CRD
make deploy-aio

# 端口转发
kubectl port-forward -n lattice-system svc/lattice-api-service 8080:8080 &

# 运行 e2e（使用默认参数）
make test-e2e

# 指定 agent 镜像
go test ./test/e2e/ -v -timeout 20m \
  --agent-image=ghcr.io/winstonfly/lattice:e2e \
  --manage-url=http://localhost:8080
```

### CI

触发条件：PR 创建/更新时自动运行（`.github/workflows/build-and-deploy.yml`）。失败时自动收集诊断日志（控制面 Pod 状态、latticed 日志、测试 namespace Pod 状态、CRD 状态）。

---

## 与 Agent Sandbox e2e 的关系

| 维度 | 基础设施 e2e（本文档） | Agent Sandbox e2e |
|------|----------------------|--------------------|
| Agent 类型 | 标准 lattice agent（privileged） | lattice-agent-sandbox（零特权） |
| 部署方式 | Deployment + `lattice up --token` | Pod + `start --wg` |
| 注册方式 | Join token | Enrollment token |
| 网络栈 | 内核 TUN (wf0) + iptables | gVisor Go netstack + wireguard-go |
| 审计 | 无 | JSONL 文件 (`/tmp/lattice-audit.jsonl`) |
| CRD | LatticePeer | LatticePeer + AgentIdentity |
| CI | 标准 e2e workflow | `sandbox-e2e.yml`（PRO only, `run-pro` label） |

两者**共享**：Ginkgo v2 框架、`helpers_test.go` 辅助函数、k3d 测试集群、latticed All-in-One 控制面。Agent Sandbox e2e 在基础设施 e2e 的 helper 基础上扩展自己的辅助函数（`createEnrollmentToken`, `deploySandboxPod`, `readAuditLog` 等）。

---

*最后更新: 2026-05-13*
