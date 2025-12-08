# Wireflow

[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Docker Pulls](https://img.shields.io/docker/pulls/wireflow/wireflow.svg?logo=docker&logoColor=white)](https://hub.docker.com/r/wireflow/wireflow)
[![Go Report Card](https://goreportcard.com/badge/github.com/wireflowio/wireflow)](https://goreportcard.com/report/github.com/wireflowio/wireflow)
![Platforms](https://img.shields.io/badge/platforms-windows%20%7C%20linux%20%7C%20macos%20%7C%20android%20%7C%20ios-informational)
[![English](https://img.shields.io/badge/lang-English-informational)](README.md) [![中文](https://img.shields.io/badge/语言-中文-informational)](README_zh.md)

## 介绍

Wireflow：您的零配置安全私有网络解决方案。

借助 WireGuard 的高性能加密能力，Wireflow 帮助您轻松构建一个跨平台的安全覆盖网络。

Wireflow 的独特之处在于其 Kubernetes 原生的控制平面：wireflow-controller 通过监听 CRD 自动完成网络编排和访问控制。用户可以通过直观的 Web UI 或声明式配置，集中管理设备连接、访问策略和整个网络状态。

核心价值： 告别复杂的 VPN 配置，实现企业级零信任网络自动化。

了解更多详情，请访问官网：[The Wireflow Authors](https://wireflow.io)

## 技术架构

- 控制平面 / 数据平面分离
- WireGuard 加密隧道（ChaCha20‑Poly1305）
- 通过控制平面进行密钥自动分发与轮换
- NAT 穿透：优先直连 P2P，不通时回退至中继（TURN）
- 节点发现与连接编排引擎
- 叠加网络内的私有 DNS 名称解析
- 指标与监控（兼容 Prometheus）
- 管理 API 与 Web UI，支持 RBAC 访问策略
- 支持 Docker 部署；`conf/` 目录提供 Kubernetes 示例与清单
- 跨平台 Agent（Linux、macOS、Windows；移动端开发中）

## 网络拓扑（高层）

- 设备通过 WireGuard 形成网状叠加网络。
- 优先进行直连 P2P；当直连不可达时，经由中继/TURN 转发流量。
- 控制平面负责设备成员、密钥与策略的管理。

## 快速开始

1. 在 Wireflow 上注册并登录。
2. 在 Web UI 中创建网络，并按向导添加设备。
3. 在每台设备上安装并运行 Wireflow 应用/Agent，使用你的账户登录并加入网络。

完成后，你应能根据所配置的访问规则，在私有网络中互相访问设备。

## 安装方式

根据你的环境选择合适的安装方式。

### Docker

```bash
docker run -d \
  --name wireflow \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  -p 51820:51820/udp \
  wireflow/wireflow:latest
```

### 二进制（一键脚本）

```bash
bash <(curl -fsSL https://wireflow.io/install.sh)
```

### 从源码构建

```bash
git clone https://github.com/wireflowio/wireflow.git
cd wireflow
make build
# 然后按需安装或运行构建产物
```

### 桌面/移动端应用

从官网获取安装包：[wireflow.io](https://wireflow.io)

## 中继（TURN）概览

当直连 P2P 失败（例如受限的 NAT 环境），Wireflow 可自动通过中继转发流量。我们提供便捷的公共中继，也支持你自建：可以使用提供的中继镜像，或运行兼容的 TURN 服务器（如 `coturn`）。

## 自建中继（示例步骤）

1. 准备一台具有公网 IP 的服务器，并开放 UDP 端口（默认 3478/5349，或你的自定义端口）。
2. 部署 Wireflow 中继镜像，或配置 `coturn`。
3. 在 Wireflow 控制平面中添加你的中继地址，便于客户端发现与使用。

更多部署示例与清单，请参考本仓库的 `conf/` 与 `turn/` 目录。

## 功能特性

- [x] 零配置上手：注册、登录、创建网络
- [x] 安全性：WireGuard 加密与密钥管理
- [x] 访问控制：定义规则策略，精确控制访问范围
- [x] Web UI：统一管理设备、规则与可见性
- [x] 中继回退：直连不可达时自动回退至中继
- [x] 跨平台：Windows、Linux、macOS、Android、iOS、NAS
- [ ] 指标：流量、连接与健康度可观测
- [ ] 多网络：统一管理多个隔离的叠加网络
- [ ] Docker UI：无需桌面应用即可管理网络
- [ ] DNS：内置服务与自定义域名支持

## 许可证

Apache License 2.0


