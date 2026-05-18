# gVisor 架构讨论：库模式 vs runsc 沙箱模式

> 2026-05-16 讨论记录，关于 Agent Sandbox 的隔离架构选型。

## 背景：gVisor 是什么

gVisor 是 Google 开源的容器沙箱，提供两个使用层次：

```
┌─ runsc（应用沙箱）─────────────┐   ┌─ netstack 库（网络组件）──────┐
│ 拦截所有系统调用                 │   │ 只替换网络协议栈               │
│ 内核 → gVisor Sentry            │   │ 其他系统调用走真实内核          │
│ 文件/网络/进程全隔离             │   │ 轻量，适合已有容器              │
│ 需要 runsc runtime              │   │ 不需要特殊 runtime             │
└─────────────────────────────────┘   └───────────────────────────────┘
```

Lattice 当前使用的是 **netstack 库模式**。

## 当前架构：netstack 库模式

AI Agent 的进程（wget/curl/nginx）跑在**普通容器里，走内核 TCP/IP**。gVisor netstack 作为一个**嵌入式的用户态网络栈**，只为 wireguard-go 服务。

### 当前数据流

```
Lattice 沙箱 Pod
  ┌── gVisor netstack（用户态）────┐
  │     ↕ channel                   │   独立协议栈，不跟内核互通
  │  wireguard-go（用户态 WG）       │
  │     ↕ UDP socket（唯一内核依赖）  │
  ├─────────────────────────────────┤
  │  内核 TCP/IP（eth0, lo）        │   另一套协议栈
  │     ↕                           │
  │  SOCKS5 proxy / ForwardListener │   ← 桥接层（翻译官）
  │     ↕                           │
  │  wget / nginx / agent tools     │
  └─────────────────────────────────┘
```

### 入向和出向

- **入向**：远端 WireGuard 加密包 → UDP socket → wireguard-go 解密 → gVisor channel（"收包"）→ netstack → ForwardListener → 转发给 nginx
- **出向**：wget → SOCKS5 proxy（127.0.0.1:1080）→ `sb.DialContext` → gVisor netstack 生成 TCP SYN → channel → wireguard-go 读包加密 → UDP 发出

### 为什么需要 proxy/ForwardListener？

因为两套网络栈互不联通：
- 应用进程（wget/nginx）走内核 TCP/IP
- WireGuard overlay 在 gVisor 用户态网络栈里

proxy 和 ForwardListener 就是**跨域的转发器**，把内核域的网络请求"翻译"到 gVisor 用户态，再走 WireGuard overlay。

### 库模式的优势

| 优点 | 说明 |
|------|------|
| **轻量** | 不需要容器 runtime 切换，不需要 containerd-shim-runsc |
| **灵活** | gVisor netstack 可以作为独立组件注入 hook（BeforeDial/AfterDial） |
| **快速迭代** | 不需要理解 runsc 的 Sentry/Gofer 机制 |
| **策略可插拔** | PolicyChecker / AuditWriter 通过简单的回调注入 |

## 未来方向：runsc 沙箱模式

如果要实现**系统调用级别的全面隔离**，应该用 runsc。

### runsc 架构

```
┌─ Lattice 沙箱 Pod（runsc runtime）──────────────────────────┐
│                                                               │
│  ┌─ gVisor Sandbox（Sentry + Gofer）──────────────────────┐  │
│  │                                                          │  │
│  │   AI agent tool calls（curl/python/cat ...）              │  │
│  │   → 所有系统调用被 gVisor 拦截                             │  │
│  │                                                          │  │
│  │   gVisor 用户态内核层：                                     │  │
│  │   ┌─ netstack（TCP/IP）──┐    ← 唯一的"网卡"               │  │
│  │   │ wireguard-go（WG）    │                               │  │
│  │   └──────────────────────┘                               │  │
│  │   ┌─ VFS（tmpfs/gofer）──┐    ← 受限文件系统               │  │
│  │   │ /tmp, /app, ...      │                               │  │
│  │   └──────────────────────┘                               │  │
│  │                                                          │  │
│  └──────────────────────────────────────────────────────────┘  │
│                          ↕ UDP :51820                          │
│              [host 内核只提供 socket 转发]                       │
└──────────────────────────────────────────────────────────────┘
```

### 关键变化

| | 库模式（当前） | runsc 模式（未来） |
|---|---|---|
| **Agent 进程跑在哪** | 真实内核上 | gVisor Sentry 上 |
| **隔离范围** | 仅网络（policy hook） | 全部：网络 + 文件 + 进程 + 系统调用 |
| **桥接层** | 需要 proxy/ForwardListener | **不需要**——agent 直连 gVisor netstack |
| **内核网络暴露** | eth0 可见，非 overlay IP 需策略拦截 | 沙箱里**没有 eth0**，只有 WireGuard |
| **出向阻断方式** | policy hook 返回错误 | gVisor 路由表**查不到**非法目标 |
| **部署依赖** | 无特殊依赖 | k8s 节点需安装 runsc |
| **性能** | 出向多一跳（proxy → gVisor） | 无跳，agent 直连 netstack |

### runsc 实现要点

**1. 容器 Runtime**

```yaml
# sandbox pod spec
spec:
  runtimeClassName: gvisor  # 需预装 runsc
  containers:
  - name: sandbox
    image: lattice-agent-sandbox:pro
```

**2. gVisor 平台配置——Netstack 是唯一 NIC**

```go
stack := stack.New(stack.Options{...})

channelNIC := channel.New(1024, 1500, "")
stack.CreateNIC(1, channelNIC)

// 所有流量都走 WireGuard——没有 eth0，没有默认路由到外部
stack.SetRouteTable([]tcpip.Route{
    {Destination: header.IPv4EmptySubnet, NIC: 1},
})

tunDev := gvisor.NewTUNAdapter(channelNIC, gvisor.InjectIntoChannel(channelNIC))
wgDev := device.NewDevice(tunDev, wgBindAdapter, wgLogger)
```

sandbox 里 `curl 10.0.2.2:8080` → gVisor TCP/IP → channel → wireguard-go → UDP，**不需要 proxy**。

**3. Lattice 增值层保持不变**

```
                   ┌─ Lattice 控制面 ──────────────────┐
                   │ AgentIdentity | Policy | Audit      │
                   └────────────────────────────────────┘
                                 ↕ NATS
┌─ gVisor runsc 沙箱 ──────────────────────────────────────┐
│  agent → gVisor netstack → wireguard-go → WireGuard UDP  │
│  BeforeDial: PolicyChecker.Allow(ip, port)               │
│  AfterDial:  AuditWriter.Write(event)                    │
└─────────────────────────────────────────────────────────┘
```

## 关于 gVisor 官方 runsc 的流量处理

gVisor 的 runsc **不**需要 proxy 处理容器间流量。原因是：

- runsc 把**整个容器进程**包裹在 gVisor Sentry 里
- 应用的所有 `socket()`/`connect()` 等系统调用都被 gVisor 拦截
- 应用到对端的流量直接走 gVisor netstack → WireGuard → 网络

对应用**完全透明**，`curl http://10.0.2.1:8080` 不需要感知底层是内核还是 gVisor。

```
runsc 容器 A                      runsc 容器 B
  ┌─ app（curl）────────────────┐   ┌─ app（nginx）──────┐
  │  ↓ socket() 被 gVisor 拦截  │   │  ↑                 │
  │ gVisor netstack（用户态）    │   │ gVisor netstack    │
  │  ↓ wireguard-go             │   │  ↑ wireguard-go    │
  │  ↓ UDP ─────────────────────┼───│ UDP                │
  └─────────────────────────────┘   └────────────────────┘
```

## 结论：演进路线

当前库模式已经验证了**网络隔离 + 策略管控 + 审计**的核心链路。runsc 模式是**隔离完整性的下一步**——把隔离从不完整的网络层提升到系统调用层，让 AI agent 跑在一个"没见过真实内核"的沙箱里，而 Lattice 的增量是给这个沙箱注入**自带零信任网络的 WireGuard overlay**。

### 设计决策：为什么先选库模式

这不是技术妥协，而是基于 **AI agent 流量特征**的务实判断：

| 特征 | AI Agent 流量 | 微服务/普通应用 |
|------|---------------|-----------------|
| 请求频率 | 几秒到分钟级（tool call） | 每秒几万 QPS |
| 数据量 | 几 KB 到几十 KB（API 响应、文件读写） | MB/GB 级别 |
| 时延敏感度 | 不敏感（LLM 推理本身就几百 ms~几 s） | 毫秒级 SLA |
| proxy 额外开销 | 几十微秒，完全淹没在 LLM 延迟里 | 不可接受的额外跳 |

**结论**：agent 之间流量小，proxy 方式的开销可以完全忽略。库模式的高灵活性和零部署依赖换来了快速迭代能力。**等 agent 场景出现持续高吞吐需求（如流式推理转发、大文件传输），再切 runsc 也不迟**——Lattice 的策略/审计层与底层沙箱实现解耦，切换成本可控。

### 演进优先级

1. **已完成**：库模式 E2E 链路验证（注册 → WireGuard 进出 → 策略 → 审计 → 撤销）
2. **短期**：修复剩余的连通性问题（`lattice status` 误导性输出、diagnostic log 收集）
3. **中期**：探索 runsc runtime 集成，实现全系统调用级别隔离
4. **长期**：沙箱 → MicroVM（Firecracker）— 硬件虚拟化级别隔离

## 参考

- [gVisor 官方文档](https://gvisor.dev/)
- [runsc 容器 runtime](https://gvisor.dev/docs/user_guide/containerd/quick_start/)
- Agent Sandbox 设计文档：`docs/superpowers/specs/2026-05-11-agent-sandbox-and-ecosystem-design.md`
- Agent Sandbox E2E 测试设计：`docs/superpowers/plans/2026-05-13-agent-sandbox-e2e-testing.md`
