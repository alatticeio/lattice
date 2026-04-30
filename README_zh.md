# Lattice - 云原生 WireGuard 网络管理平台

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/alatticeio/lattice)](https://goreportcard.com/report/github.com/alatticeio/lattice)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

## 项目简介

**Lattice：基于 Kubernetes CRDS 设计的云原生网络编排方案。**

Lattice 旨在简化跨云、跨数据中心以及边缘设备的 覆盖网络 (Overlay Network) 构建。它通过 Kubernetes 原生方式，自动化管理 WireGuard 隧道的建立与配置。

* **控制面 (Control Plane)**：基于 Kubernetes Operator 模式，通过自定义资源 (CRD) 声明式地定义网络拓扑，是集群状态的“大脑”。
* **数据面 (Data Plane)**：轻量级 Agent 部署，实现设备间的高性能 P2P 隧道连接。它具备强大的 NAT 穿透能力，确保护网状态的最终一致性。

了解更多信息，请访问官方网站：[lattice.run](https://lattice.run)

---

## 核心特性

### 架构与核心安全

* **解耦架构**：控制平面负责决策，数据平面负责转发，确保单点故障不影响已有隧道的连通性。
* **高性能隧道**：强制使用 WireGuard (ChaCha20-Poly1305) 协议，提供极致的传输性能与安全性。
* **零接触密钥管理**：自动化的密钥分发与轮换，所有配置由控制面完成，实现零接触配置（Zero-Touch Provisioning）。

### Kubernetes 原生集成

* **声明式 API**：像管理 Pod 一样管理你的私有网络。
* **自动化 IPAM**：内置 IP 地址管理系统，自动为租户和节点分配互不冲突的私有 IP。
* **智能拓扑编排**：利用 Kubernetes Label 自动发现节点并编排 Mesh 或 Star 型网络拓扑。

---

## 快速上手

### 一键本地部署（推荐）

最快速地在本地运行完整 Lattice 控制面，只需安装 **Docker** — 脚本会自动安装 k3d 和 kubectl。

```bash
curl -sSL https://raw.githubusercontent.com/alatticeio/lattice/master/hack/quickstart.sh | bash
```

脚本执行流程：
1. **前置检查**：验证 Docker、k3d、kubectl 是否就绪（缺失工具自动安装）。
2. **端口检查**：确认 **8080**（Dashboard/API）和 **4222**（NATS 信令）端口空闲。
3. **创建集群**：建立名为 `lattice` 的 k3d 集群，并自动完成宿主机端口映射。
4. **顺序加载**：按 CRDs → RBAC → Service → Deployment 的顺序部署资源。
5. **健康探测**：等待 Pod 就绪并探测 API 健康接口。
6. **打印连接串**：输出包含 NATS 地址和初始 Token 的 **Agent 一键接入命令**。

脚本执行完毕后，在浏览器打开控制台：

```
http://localhost:8080
```

### 安装控制面（使用已有集群）

如果你已有配置好 `kubectl` 的 Kubernetes 集群：

```bash
curl -sSL https://raw.githubusercontent.com/alatticeio/lattice/master/hack/install-k3d.sh | bash
```

### 安装数据面 (Agent)

```bash
curl -sSL https://raw.githubusercontent.com/alatticeio/lattice/master/hack/install.sh | bash
```

#### 使用 Docker 运行 Agent

```bash
docker run -d --name lattice --restart=always ghcr.io/alatticeio/lattice:latest up
```

## 令牌 (Token) 管理

Lattice 使用基于 Token 的认证系统安全管理节点入网授权。如果还没有 Token，可以创建一个：

```bash
lattice token create dev-team \
  --signaling-url nats://localhost:4222 \
  -n test --limit 5 --expiry 168h
```

参数说明：
- `dev-team`：令牌名称
- `test`：令牌作用的命名空间
- `5`：该令牌允许的最大并发连接数
- `168h`：令牌有效期

### 使用令牌接入网络

```bash
lattice up --signaling-url nats://localhost:4222 --token <token>
```

在 Docker 中运行：

```bash
docker run -d --name lattice --restart=always \
  ghcr.io/alatticeio/lattice:latest up \
  --signaling-url nats://localhost:4222 --token <token>
```

### 在控制面查看节点

```bash
kubectl get wfpeer -n test
```

当另一个节点加入网络时，它会自动与网络中的其他节点建立连接，节点之间自动组网成功。

## 卸载

移除控制面并清理本地 k3d 集群：

```bash
k3d cluster delete lattice
```

## 开发指南

### 环境
参照上边创建一个k3d的环境

### 从源码构建

```bash
git clone [https://github.com/alatticeio/lattice.git](https://github.com/alatticeio/lattice.git)
cd lattice
make build-all
```

## 徽章 (Badges)

### 贡献者

## Lattice 特性与愿景

Lattice 的架构专注于 自动化 (Automation) 与 零信任安全 (Zero-Trust Security)。

### 核心特性 (已实现)
- 零接触组网 (Zero-Touch Networking)：自动设备注册与配置，无需手动维护 WireGuard 隧道。
- K8s 原生编排：基于 CRD 设计，利用 Kubernetes 节点标签 (Labels) 实现自动化的设备发现与连接调度。
- 安全加固：基于 WireGuard 内核加密，控制面中心化管理密钥分发与轮换。
- 灵活的网络能力：内置 IPAM 自动分配地址，提供声明式的访问策略模型 (ACL)。

### 未来里程碑 (计划中)

- 我们致力于构建全球规模的云原生加密网络。
- 跨云与多地域：支持混合云部署，打通不同云厂商与物理区域的网络孤岛。
- 多租户与权限：支持多租户隔离，并集成 RBAC 与中心化 Web 管理界面。
- 运维可视化：内置 Prometheus 指标导出器，提供流量监控与告警功能。
- 智能服务发现：集成内置 DNS，为私有网络提供安全的服务发现机制。

## 免责声明 (Disclaimer)

- 本工具仅限于技术研究、企业内网互联、合规的远程办公等合法场景。
- 用户在使用本软件时，必须遵守当地法律法规。
- 严禁将本工具用于任何违反《中华人民共和国网络安全法》及相关法律的行为（包括但不限于建立非法跨境信道）。
- 作者不对用户利用本工具进行的任何违法行为承担法律责任。

## 开源协议

基于 Apache License 2.0 协议。