Project Context: Wireflow

1. 项目愿景 (Vision)
   Wireflow 是一个 Kubernetes 原生的网络编排平台，专注于管理跨云、跨地域以及边缘节点的 WireGuard 隧道。它旨在提供简单、安全且具备自愈能力的
   Overlay 网络底座。

2. 核心技术栈 (Technical Stack)
   语言: Go (Golang) 1.24+

编排: Kubernetes (Operator SDK / controller-runtime)

数据面: WireGuard (Native Linux kernel module / wg-quick)

通信层: * 集群内: K8s API (In-cluster config)

边缘端: NATS JetStream (异步状态同步)

前端: Vue 3 + Tailwind CSS + DaisyUI

3. 架构拓扑 (Architecture)
   Control Plane: 运行在 K8s 集群中，管理 WireguardPeer 和 WireflowNetwork 等 CRD。

Edge Agent: 运行在边缘节点（无 K8s 环境），通过 NATS 订阅配置并操作本地 wg 接口。

Shadow State: 使用 NATS 作为影子状态存储，解决边缘节点离线后的配置对齐问题。

4. 关键 CRD 设计逻辑
   WireflowPeer: 定义一个端点，包含 PublicKey、AllowedIPs 和 Endpoint 信息。

Status Management: 必须包含 Ready, Connected, LastHandshake 等条件（Conditions）。

Finalizers: 所有的 CR 必须实现 Finalizer，以确保在删除时清理物理接口、路由表及 NATS 中的持久化消息。

5. 开发准则 (Invariants)
   Idempotency First: 所有的 Reconcile 逻辑必须是幂等的。

Zero-Trust: 默认拒绝所有流量，仅允许 AllowedIPs 定义的范围。

Dependency Minimization: 保持 Agent 极其轻量，严禁引入非必要的第三方库。

Testing: 核心逻辑必须配套 _test.go，涉及网络变动的必须通过 .ai/blueprints/e2e-test-spec.md 定义的 E2E 测试。

6. 当前开发进度 (Current Status)
   [x] 基础 Operator 框架搭建
   [x] 实现了隧道打通
   [x] 实现了CRD

[x] WireguardPeer CRD 核心逻辑 (v0.1.0)

[ ] NATS 集成与边缘 Agent 基础版本 (当前重心)

[ ] 监控看板 (Vue 3) 基础指标展示