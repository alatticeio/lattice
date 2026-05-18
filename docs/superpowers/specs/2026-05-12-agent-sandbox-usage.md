# Lattice Agent Sandbox 使用指南

> 最后更新：2026-05-18
> 关联设计: `docs/superpowers/specs/2026-05-16-agent-platform-integrated-design.md`

## 概述

`lattice sandbox` 是 Lattice 内置的零特权 AI Agent 沙箱。它将 gVisor 用户态网络栈与 WireGuard overlay 融合，让 Agent 以普通用户权限运行，同时获得完整的 Lattice 网络身份（NATS 注册 + ICE 打洞 + LRP relay 回退），与基础设施节点的行为完全一致。

**Community 版**：gVisor 网络隔离 + 本地文件审计（`/tmp/lattice-audit-<name>.jsonl`）
**PRO 版**：新增出站策略过滤（EgressFilter）、入站端口转发（ForwardListener）、HTTP 正向代理、NATS 中心化审计上报

---

## 快速开始

### 1. 在控制面创建 enrollment token

```bash
curl -X POST http://localhost:8080/api/v1/agent-isolation/enrollment-tokens \
  -H "Authorization: Bearer <user-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "default",
    "allowedTools": ["list_peers", "list_policies", "check_connectivity"],
    "ttlSeconds": 3600
  }'
```

响应：
```json
{
  "code": 200,
  "data": {
    "Token": "lt-enroll-a1b2c3...",
    "ExpiresAt": "2026-05-18T14:00:00Z"
  }
}
```

### 2. 启动 sandbox agent

```bash
lattice sandbox start \
  --name my-agent \
  --server-url http://localhost:8080 \
  --token lt-enroll-a1b2c3...
```

启动流程：
1. 检查 `/etc/lattice/sandbox-credentials.json`（容器重启恢复路径）
2. 若无缓存凭证 → 生成 WireGuard 密钥对 → NATS 注册（发送 enrollment token + 公钥）→ 服务端创建 `LatticePeer` + `AgentIdentity`，返回 Agent JWT
3. 轮询 NATS `GetNetMap`（最多 60 s）直到 VPN IP 分配完成
4. 持久化凭证到 `/etc/lattice/sandbox-credentials.json`（重启免重注册）
5. 若控制面返回 relay URL → 自动启用 LRP relay
6. 创建 gVisor sandbox（`internal/agent/gvisor`）
7. 创建 gVisor TUNAdapter，传入 `agent.NewNode`（与普通节点共享 NATS + ICE + LRP 基础设施）
8. 启动 30 s 心跳（保持 sandbox 在其他节点的 ComputedPeers 中）
9. 启动 15 s 周期刷新（兜底 NATS push 丢失）
10. 阻塞等待 SIGINT/SIGTERM

### 3. 验证连通性

```bash
# 在 companion 节点
ping <sandbox-overlay-ip>

# 在 sandbox 容器内（出站走 gVisor overlay）
curl --proxy socks5://127.0.0.1:1080 http://10.100.0.2:8080   # PRO: HTTP proxy
```

---

## 命令行参考

```
lattice sandbox start [flags]

必填:
  --name string         Sandbox 标识符
  --server-url string   Lattice 控制面 URL
  --token string        Enrollment token（首次启动）

PRO 专属:
  --proxy-addr string          HTTP 正向代理监听地址（如 127.0.0.1:1080）
  --forward stringArray        入站端口转发规则：overlayPort:targetAddr
  --egress-allow string        允许出站的 CIDR 列表（逗号分隔）
  --egress-default-deny        启用出站白名单模式（默认放行）
```

> Community 版不支持 `--proxy-addr`、`--forward`、`--egress-allow`、`--egress-default-deny`。

---

## 网络架构

```
                    ┌─────────────────────────────┐
                    │       gVisor Sandbox         │
                    │                              │
  Agent 进程  ──▶   │  gVisor netstack (pkg/tcpip) │
  connect()         │        │                    │
                    │  tunAdapter (RW loop)        │
                    │        │                    │
                    │  wireguard-go Device         │
                    └──────────┬──────────────────┘
                               │ UDP :51820
                    ┌──────────▼──────────────────┐
                    │   FilteringUDPMux            │
                    │   STUN ──▶ ICE agent        │
                    │   non-STUN ──▶ WG DefaultBind│
                    └──────────┬──────────────────┘
                               │
              ┌────────────────┴──────────────┐
              │  ICE 打洞成功                  │  ICE 失败
              ▼                               ▼
        Direct P2P                    LRP relay (QUIC/TCP)
```

Sandbox 走的是与普通节点**完全相同**的信令路径：`NATS → ProbeFactory → ICE/LRP`。gVisor 只负责替换内核 TUN 设备，上层逻辑无感知。

---

## 凭证持久化

Sandbox 在首次注册后将 WireGuard 私钥和 Agent JWT 写入：

```
$LATTICE_CONFIG_DIR/sandbox-credentials.json   # 默认 /etc/lattice/sandbox-credentials.json
```

权限：`0600`。容器重启后，sandbox 优先读取该文件跳过重注册（`ResumeSandboxViaNATS`），仅在 JWT 失效或文件不存在时才消耗 enrollment token。

---

## 审计日志

### Community（本地文件）

每次出站连接写一条 JSONL 到：

```
/tmp/lattice-audit-<name>.jsonl
```

示例：
```json
{"identity":"my-agent","dst_ip":"10.100.0.2","dst_port":443,"protocol":"tcp","verdict":"allow"}
```

### PRO（NATS 上报 → 控制面）

PRO 版 `natsAuditWriter` 将事件发布到 `lattice.audit.flow`，控制面订阅后写入 `flow_events` 表，可通过 `/api/v1/agent-isolation/audit/traces` 查询并与工具调用 trace 关联。

---

## 工具调用可观测性（MCP Trace）

每次 Agent 调用工具时，控制面自动记录 `tool_spans`：

| 字段 | 说明 |
|------|------|
| `traceID` | UUID，全局唯一 |
| `agentID` | 来自 JWT claims |
| `parentID` | sub-agent 场景下的父 AgentID |
| `tool` | 工具名称 |
| `status` | `ok` / `error` / `blocked` |
| `durationMs` | 耗时 |

查询 API：
```
GET /api/v1/agent-isolation/audit/traces?agentId=xxx&from=RFC3339&to=RFC3339
GET /api/v1/agent-isolation/audit/traces/:traceId
GET /api/v1/agent-isolation/audit/agents/:agentId/calltree
```

---

## Sub-agent（委派）

父 Agent 可通过 Delegate API 派生子 Agent，子 Agent 获得独立 WireGuard 身份，权限不超过父级：

```bash
# 父 Agent 请求子 Agent enrollment token
curl -X POST http://localhost:8080/api/v1/agent-isolation/delegate \
  -H "Authorization: Bearer <parent-agent-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "agentName": "sub-executor-001",
    "requestedTools": ["exec", "read"],
    "ttlSeconds": 900
  }'
```

响应：
```json
{
  "enrollmentToken": "lt-enroll-sub-xxx",
  "expiresAt": "2026-05-18T14:15:00Z"
}
```

子 Agent 用此 token 正常调用 `lattice sandbox start`，启动后：
- `AgentIdentity.Spec.ParentRef` = 父 AgentID
- Agent JWT 中携带 `parent_agent_id` claim
- 工具调用 trace 可还原完整调用树

---

## API 接口

### 管理面（需用户 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agent-isolation/enrollment-tokens` | 创建一次性 enrollment token |
| DELETE | `/api/v1/agent-isolation/agents/:name?namespace=` | 吊销 Agent（Patch 状态为 Revoked） |
| GET | `/api/v1/agent-isolation/audit/traces` | 查询工具调用 trace |
| GET | `/api/v1/agent-isolation/audit/traces/:traceId` | 查询单条 trace |
| GET | `/api/v1/agent-isolation/audit/agents/:agentId/calltree` | 查询 sub-agent 调用树 |

### Agent 面（需 Agent JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agents/tools/call` | 工具调用（RBAC 检查 + trace 记录） |
| POST | `/api/v1/agent-isolation/delegate` | 派生子 Agent enrollment token |

> `POST /api/v1/agent-isolation/register` 现在是公开端点（无需 JWT），供 sandbox 内部 NATS 注册流程使用。

---

## Community vs PRO 对比

| 能力 | Community | PRO |
|------|-----------|-----|
| gVisor 网络栈（零特权 WireGuard） | ✅ | ✅ |
| NATS 注册 + ICE 打洞 + LRP relay | ✅ | ✅ |
| 凭证持久化（重启免重注册） | ✅ | ✅ |
| 本地文件审计 | ✅ | ✅ |
| 工具调用 trace（tool_spans） | ✅ | ✅ |
| Sub-agent Delegate API | ✅ | ✅ |
| 出站策略过滤（EgressFilter） | ❌ | ✅ |
| 入站端口转发（ForwardListener） | ❌ | ✅ |
| HTTP 正向代理（`--proxy-addr`） | ❌ | ✅ |
| NATS 中心化审计上报（flow_events） | ❌ | ✅ |

---

## Sandbox vs 普通节点对比

| 维度 | 普通节点（`lattice up`） | Sandbox（`lattice sandbox start`） |
|------|------------------------|----------------------------------|
| 隔离方式 | 无（宿主机进程） | gVisor 用户态网络栈 |
| 特权需求 | root / CAP_NET_ADMIN | **零**（普通用户） |
| 网络栈 | 内核 TUN (wf0) | gVisor `pkg/tcpip` + tunAdapter |
| WireGuard | 内核 WireGuard + FilteringUDPMux | wireguard-go + FilteringUDPMux |
| 注册方式 | NATS（LatticeEnrollmentToken） | NATS（AgentEnrollmentToken + AgentIdentity） |
| ICE / LRP | ✅ 完整支持 | ✅ 完整支持（共享同一套基础设施） |
| 策略执行 | eBPF TC (PRO) / iptables | Go 层 EgressFilter（PRO） |
| 审计 | eBPF ring buffer (PRO) | Go 层 AuditWriter |

---

## 构建

```bash
# Community 版（gVisor 可用，无出站策略/代理/转发）
make build SERVICE=lattice

# PRO 版（完整功能）
make build SERVICE=lattice EDITION=pro
```

---

## 限制

- gVisor netstack 实现了 ~95% 常用 syscall；依赖 `io_uring`、`AF_PACKET` 等的应用可能不兼容
- 出站策略（`--egress-allow` / `--egress-default-deny`）仅 PRO 版支持
- gVisor 只隔离经过 sandbox 代理/转发的流量；AI 进程直接通过 `eth0` 发出的流量不受控（需配合 netns 或 iptables 封堵）
