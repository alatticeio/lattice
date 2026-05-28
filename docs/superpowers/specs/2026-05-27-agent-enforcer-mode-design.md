---
title: Agent Enforcer Mode Selection
---

# Agent Enforcer Mode Selection

> Source: `cmd/lattice/cmd/up.go`, `internal/agent/provision/`, `internal/agent/config/config.go`, `internal/agent/node.go`, `internal/server/server/api.go`

## Problem (现状)

Lattice agent 的策略执行后端（iptables / eBPF）完全由代码自动决定，用户和平台管理员无法控制：

```
SelectEnforcerMode()
 └─ selectEBPAvailable()
      社区版 (!pro) → 永远 ModeIPTables
      PRO版 (pro)  → 探测内核 SCHED_CLS → ModeEBPF 或 ModeIPTables
```

三个具体痛点：

1. **用户不可控**：没有 CLI flag 或配置项，全自动决定
2. **社区版与 iptables 强绑定**：即便将来社区版也能用 eBPF，代码写死了
3. **没有 server 端统一管控**：平台管理员无法集中配置 agent 的策略执行方式

## Design Philosophy: 平台配置 vs 用户配置

| 类别 | 配置项 | 作用域 | 示例 |
|------|--------|--------|------|
| **平台基础设施** | NATS 地址、STUN 地址 | 全局，所有用户相同 | `nats_url`, `stun_url` |
| **用户策略偏好** | 策略执行后端 | 分用户，按 license 区分 | `enforcer_mode` |

- NATS/STUN = 机房运维关心的基础设施，平台管理员在 server config 里配置，discovery API 统一返回
- Enforcer mode (iptables vs eBPF) = 用户级特性，PRO 用户可以用 eBPF，社区用户只能用 iptables。所以存在用户个人设置里，agent 注册时按用户身份下发

### Decision Flow (四层优先级)

```
--enforcer-mode flag set (auto / iptables / ebpf)?
 ├─ yes, explicit → 使用指定值，ebpf 不可用时自动回退并 warning
 └─ no (auto / empty)
      └─ 注册响应中携带了用户个人设置 enforcer_mode?
           ├─ yes → 使用用户设置值，同样回退验证
           └─ no  → discovery API 返回了 server 全局默认 enforcer_mode?
                     ├─ yes → 使用 discovery 值，同样回退验证
                     └─ no  → auto-detect (build tag 探测，现有逻辑)
```

```
CLI flag > 用户个人设置 > server 全局默认 > auto-detect
```

**核心不变式**：agent 永远不会因 enforcer mode 不可用而启动失败，始终以最佳可用后端启动并记录日志。

### 1. CLI Flag

```bash
lattice up --enforcer-mode auto      # 默认，自动探测
lattice up --enforcer-mode iptables  # 强制 iptables
lattice up --enforcer-mode ebpf      # 优先 eBPF，不可用时回退 iptables
```

文件：`cmd/lattice/cmd/up.go` — 新增 `--enforcer-mode` flag，值写入 `config.Conf.EnforcerMode`，`--save` 正常工作。

### 2. Config 字段

文件：`internal/agent/config/config.go`

`Config` 结构体新增 `EnforcerMode string` 字段，`setDefaults()` 设为 `"auto"`。

```go
type Config struct {
    // ...existing fields...
    EnforcerMode string `mapstructure:"enforcer-mode"` // "auto", "iptables", "ebpf"
}
```

### 3. Server 全局默认 — Discovery API

文件：`internal/server/server/api.go` — `handleDiscovery()`

`GET /api/v1/discovery` 返回新 `enforcer_mode` 字段，值来自 server config 文件（无认证端点，全局默认）：

```go
enforcerMode := s.cfg.EnforcerMode
if enforcerMode == "" {
    enforcerMode = "auto"
}
```

响应示例：

```json
{
  "data": {
    "nats_url": "nats://10.0.0.1:4222",
    "stun_url": "stun.alattice.io:3478",
    "enforcer_mode": "ebpf"
  }
}
```

Server config（`deploy/lattice.prod.yaml` / helm values）：

```yaml
enforcer-mode: auto  # 平台管理员可设为 auto / iptables / ebpf
```

### 4. 用户个人设置 — 注册时下发

DB 存储：用户 settings（非 `system_configs`），`GET/PUT /api/v1/user/settings` 新增 `enforcer_mode` 字段。

```json
{
  "enforcer_mode": "iptables"
}
```

Agent 通过 NATS 注册流程获取用户偏好。Server 在 NATS register 响应中根据 token 查找到对应 workspace 的 owner，读取其个人设置中的 `enforcer_mode`，嵌入注册响应返回给 agent。

文件变更：

| 操作 | 文件 |
|------|------|
| 用户 settings API 返回 `enforcer_mode` | `internal/server/controller/profile.go` |
| NATS register 响应携带 `enforcer_mode` | `internal/server/nats/` 或 `internal/server/service/peer.go` |
| Agent 端 `infra.Peer` 接收 `enforcer_mode` | `internal/agent/infra/message.go` |

### 5. Selector 逻辑

文件：`internal/agent/provision/enforcer_selector.go`

```go
func SelectEnforcerMode(cfg *Config, logger *log.Logger) EnforcerMode {
    switch cfg.EnforcerMode {
    case "iptables":
        return ModeIPTables
    case "ebpf":
        if mode := selectEBPAvailable(); mode == ModeEBPF {
            return ModeEBPF
        }
        logger.Warn("ebpf requested but unavailable, falling back to iptables")
        return ModeIPTables
    default: // "auto" or empty
        return selectEBPAvailable()
    }
}
```

`selectEBPAvailable()` 保持现有行为不变（build tag 分支）。

### 6. Agent 启动集成

文件：`internal/agent/node.go`

```go
type discoveryResult struct {
    NatsURL      string
    StunURL      string
    EnforcerMode string // new: server 全局默认
}
```

优先级应用逻辑（在注册完成后）：

```go
// Step 1: apply discovery global default
if config.Conf.EnforcerMode == "" || config.Conf.EnforcerMode == "auto" {
    config.Conf.EnforcerMode = discoveryResp.EnforcerMode
}
// Step 2: apply user personal setting from registration response
if registeredPeer.EnforcerMode != "" {
    config.Conf.EnforcerMode = registeredPeer.EnforcerMode
}
// Step 3: ensure non-empty
if config.Conf.EnforcerMode == "" {
    config.Conf.EnforcerMode = "auto"
}
```

## Data Model 总览

```
┌──────────┐     ┌──────────────┐     ┌─────────────────┐
│ CLI flag │────▶│ config.Conf  │────▶│ SelectEnforcer  │
│ (up.go)  │     │ .EnforcerMode│     │ Mode(cfg, log)  │
└──────────┘     └──────┬───────┘     └────────┬────────┘
                        │                      │
              ┌─────────▼────────┐             │
              │ NATS register    │             ▼
              │ (用户个人设置)    │    ┌─────────────────┐
              │ infra.Peer       │    │ PolicyEnforcer  │
              │ .EnforcerMode    │    │ iptables | ebpf │
              └────────┬─────────┘    └─────────────────┘
                       │
              ┌────────▼─────────┐
              │ discovery API    │
              │ (server全局默认)  │
              │ /api/v1/discovery│
              └──────────────────┘
```

## Fallback 矩阵

| 请求值 | 社区版 | PRO + 内核 ≥ 5.10 | PRO + 旧内核 |
|--------|--------|-------------------|-------------|
| `auto` | iptables | ebpf | iptables |
| `iptables` | iptables | iptables | iptables |
| `ebpf` | warning → iptables | ebpf | warning → iptables |

所有回退场景 agent 都会打印 warning 日志并正常启动。

## 日志输出

```
# 用户个人设置指定 eBPF
INFO enforcer mode: ebpf (source: user-setting)

# server 全局默认
INFO enforcer mode: ebpf (source: discovery)

# CLI flag 显式指定
INFO enforcer mode: iptables (source: cli-flag)

# ebpf 因能力不足回退
WARN ebpf requested but unavailable, falling back to iptables (reason: kernel SCHED_CLS not supported)

# 纯 auto
INFO enforcer mode: iptables (source: auto)
```

## 改动清单

| 层 | 文件 | 改动 |
|----|------|------|
| CLI | `cmd/lattice/cmd/up.go` | 新增 `--enforcer-mode` flag |
| Config | `internal/agent/config/config.go` | 新增 `EnforcerMode string` 字段 |
| Selector | `internal/agent/provision/enforcer_selector.go` | `SelectEnforcerMode` 接收 Config |
| Agent discovery | `internal/agent/node.go` | `discoveryResult` + `EnforcerMode` |
| Agent register | `internal/agent/infra/message.go` | `Peer` 新增 `EnforcerMode` 字段 |
| Discovery (server) | `internal/server/server/api.go` | `handleDiscovery()` 返回 `enforcer_mode` |
| Register (server) | `internal/server/service/peer.go` | NATS register 响应中携带用户 `enforcer_mode` |
| User settings | `internal/server/controller/profile.go` | 读取/写入用户 `enforcer_mode` |
| User settings model | `internal/server/models/settings.go` | 新增 `EnforcerMode` 字段 |
| Helm values | `deploy/charts/lattice/values.yaml` | `config` 下新增 `enforcerMode` |
| Helm cm | `deploy/charts/lattice/templates/configmap.yaml` | 映射 `config.enforcerMode` → `enforcer-mode` |
