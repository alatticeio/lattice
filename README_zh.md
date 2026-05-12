# Lattice — AI 原生 WireGuard 覆盖网络

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/alatticeio/lattice)](https://goreportcard.com/report/github.com/alatticeio/lattice)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

## 项目简介

**Lattice：基于 eBPF + WireGuard 底座，同时驱动"基础互联"与"智能体安全"两个核心引擎的云原生覆盖网络平台。**

Lattice 构建身份感知（Identity-Aware）的虚拟化网络层，在一个统一底座上实现两件事：

| 引擎 | 目标 | 核心能力 |
|------|------|----------|
| **网络编排** | 解决跨云、跨 NAT 的异构节点自动化互联 | WireGuard 隧道自动化、ICE/STUN NAT 穿透、LRP 中继、K8s CRD 拓扑编排 |
| **Agent 编排** | 实现意图驱动的 AI Agent 运行时安全隔离与流量审计 | MCP 协议、身份注册、eBPF 策略执行、意图引擎、Agent 沙箱 |

了解更多：[lattice.run](https://lattice.run)

---

## 核心架构

Lattice 采用"控制面与数据面分离"的标准分布式架构，自托管部署：

```
┌────────────────────────────────────────────────────────────┐
│                    Lattice 四大平面                          │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐ │
│  │ 控制面   │  │ 数据面   │  │ 中继面   │  │  AI 面     │ │
│  │          │  │          │  │          │  │           │ │
│  │ latticed │  │ lattice  │  │  lrper   │  │ MCP 服务器 │ │
│  │ Manager  │  │ (Agent)  │  │ (TCP/    │  │ Intent     │ │
│  │ (K8s Op) │  │          │  │  QUIC)   │  │ Engine     │ │
│  │ NATS     │  │ WireGuard│  │          │  │ Agent 隔离 │ │
│  │ SQLite   │  │ ICE/STUN │  │          │  │ Python SDK │ │
│  │ Gin API  │  │ eBPF     │  │          │  │           │ │
│  └──────────┘  └──────────┘  └──────────┘  └───────────┘ │
│                                                            │
│  部署模式: All-in-One (单进程) / K8s Operator / Standalone  │
│  数据库: SQLite (默认) / MySQL                               │
│                                                             │
└────────────────────────────────────────────────────────────┘
```

### 核心组件

| 组件 | 技术选型 | 理由 |
|------|----------|------|
| 开发语言 | Go | 并发能力、高性能后台与控制器 |
| 隧道协议 | WireGuard (内核态) | 加解密极快、配置极简 |
| 内核策略 | eBPF (Cilium/ebpf-go) | 无侵入流量监控与硬阻断，优于 iptables |
| NAT 穿透 | pion/ice v4 + STUN | P2P 直连，对称 NAT 回退 Relay |
| 信令通讯 | NATS (JetStream) | 低延迟、双向通讯，无需 gRPC |
| 前端 | Vue 3.5 + Vite + Tailwind 4 | 现代化 Web 管理面板 |
| AI 集成 | MCP 协议 + LLM (Claude/DeepSeek/OpenAI) | AI 助手直接管理网络 |

---

## 核心特性

### 已实现

**基础互联**
- **自动化 WireGuard 网格**：密钥分发、IP 分配、隧道建立全程自动化
- **无感知 NAT 穿透**：ICE/STUN 实现 P2P 直连，失败自动切换 LRP 中继
- **动态拓扑**：支持星型、网状、树状拓扑，NATS 信令保证全网配置一致性
- **内置 IPAM**：按 Workspace 自动分配无冲突私有 IP
- **多平台**：Linux (iptables/eBPF)、macOS (pfctl)、Windows

**网络策略**
- **策略引擎**：声明式 allow/deny 规则，支持 IP/端口/协议级过滤
- **双模式执行**：社区版 iptables/pfctl，PRO 版 eBPF TC 程序（内核级硬阻断）
- **默认拒绝**：多租户环境下防止意外暴露
- **TTL 策略**：临时策略自动过期

**多租户与安全**
- **Workspace 隔离**：每个 Workspace 映射独立 K8s Namespace
- **RBAC 权限**：admin / editor / member / viewer 四级角色
- **JWT 认证**：用户 JWT + Agent JWT，支持吊销列表
- **全功能 Web 面板**：仪表盘、Peer 管理、策略编辑器、告警

**AI 原生**
- **MCP 服务器**：Claude Desktop、Cursor 等 AI 助手通过自然语言管理网络
- **网络意图引擎**：自然语言 → CRD 变更计划 → diff 预览 → 审批执行（PRO）
- **AI Agent 网络**：零信任注册（TTL 身份 + 网络隔离预设），Python SDK
- **AI 对话面板**：Web UI 内嵌 SSE 流式 AI 对话
- **合规审计**：workspace 级别的安全策略审计

### Roadmap（设计完成，待实现）

**Phase 1 — Agent 运行时沙箱（社区版）** ([设计文档](docs/superpowers/specs/2026-05-11-agent-sandbox-and-ecosystem-design.md))
- `lattice-agent-sandbox`：标准化 Agent 执行环境，cgroup 隔离
- PID ↔ TUN 绑定：eBPF `cgroup/connect4` 强制进程流量走 WireGuard
- Sidecar 意图拦截：seccomp notify，零侵入拦截 Agent 外联

**Phase 2 — 零特权深度隔离（PRO）**
- **gVisor (runsc) 沙箱**：OCI 原生，无需 TUN、无需 eBPF、无需 root
  - wireguard-go 直接附着 gVisor netstack link endpoint，纯用户态闭环
  - Sentry 层 socket 拦截替代内核 eBPF TC hook
- **`lattice-shim`**：独立 Go 库，gVisor netstack ⟷ wireguard-go 桥接，零 Lattice 依赖
- **eBPF 流量镜像**：保留用于内核路径基础设施节点（高性能策略场景）

**Phase 3 — 生态与可视化（社区版）**
- 全局拓扑图：D3.js 力导向图，P2P/LRP 连接路径实时展示
- 个人玩家模式：`lattice quickstart` 一键启动，零配置自建 Tailscale

> **eBPF 定位说明**：引入 gVisor 后策略执行分为双轨——AI Agent 走 gVisor Sentry 纯用户态
> 策略，基础设施节点继续走内核 TUN + eBPF TC ingress（PRO）或 iptables（社区版）。
> eBPF 仍是 Lattice PRO 的核心差异化能力，只是不用于 Agent 沙箱路径。

---

## 快速开始

### Docker (单命令，无需 Kubernetes)

```bash
docker run -d \
  --name lattice-k3s \
  --privileged \
  -p 8080:8080 \
  ghcr.io/alatticeio/lattice-k3s:latest
```

约 30 秒后：
- 控制台/API：`http://localhost:8080`

### 已有 K8s 集群

```bash
kubectl apply -k https://github.com/alatticeio/lattice/config/lattice/overlays/all-in-one
```

---

## AI 功能对比

| 能力 | Lattice | Tailscale | Netbird | ZeroTier |
|---|---|---|---|---|
| MCP Server (AI 助手自然语言管理网络) | ✅ 内置 | ❌ | ❌ | ❌ |
| AI Agent 零信任注册 (TTL + 网络隔离预设) | ✅ API + SDK | ❌ | ❌ | ❌ |
| 网络意图引擎 (自然语言 → 变更计划) | ✅ | ❌ | ❌ | ❌ |
| 写操作审批工作流 | ✅ 内置 | N/A | N/A | N/A |
| eBPF 内核级流量策略 | ✅ (PRO) | ❌ | ❌ | ❌ |
| Agent 运行时沙箱 (PID/TUN 绑定) | 🔜 | ❌ | ❌ | ❌ |
| 合规对话 (Compliance-as-Conversation) | 🔜 | ❌ | ❌ | ❌ |

---

## 安装

### Homebrew (macOS / Linux)

```bash
brew tap alatticeio/tap
brew install lattice
```

### 二进制下载

从 [GitHub Releases](https://github.com/alatticeio/lattice/releases) 下载：

```bash
VERSION=$(curl -s https://api.github.com/repos/alatticeio/lattice/releases/latest | grep tag_name | cut -d'"' -f4)
curl -sSL "https://github.com/alatticeio/lattice/releases/download/${VERSION}/lattice_${VERSION}_linux_amd64.tar.gz" | tar xz
sudo mv lattice /usr/local/bin/
```

---

## 开发

### 环境要求

- Go 1.25+
- Docker 20.10+
- k3d 5.x+ (本地集群)
- kubectl 1.20+

### 从源码构建

```bash
git clone https://github.com/alatticeio/lattice.git
cd lattice
make build-all
```

### 前端开发

```bash
cd fronted && pnpm install && pnpm dev
```

---

## 免责声明

- 本工具仅限于技术研究、企业内网互联、合规的远程办公等合法场景
- 用户在使用本软件时，必须遵守所在地法律法规
- 严禁将本工具用于任何违法行为
- 作者不对用户利用本工具进行的任何违法行为承担法律责任

## 开源协议

[Apache License 2.0](LICENSE)
