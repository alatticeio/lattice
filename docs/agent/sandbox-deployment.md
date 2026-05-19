---
title: Agent Sandbox — Deployment Guide
---

# Agent Sandbox Deployment Guide

> 三种部署模式：裸机二进制、Docker Compose、Kubernetes Sidecar。

## Prerequisites

- **Lattice control plane** 已运行（`latticed` 或 Helm chart 部署）
- **Enrollment token**（通过 Dashboard → Agent → Create Token 获取，格式 `lt-xxx`）
- **Pro 版本**才支持 `--proxy-addr`、`--forward`、`--egress-allow` 参数

## Mode 1: Binary（裸机，不推荐）

直接在主机上运行 `lattice sandbox start`，AI agent 进程通过 SOCKS5 本地代理接入 overlay。

::: warn
**不推荐生产使用。** 裸机模式下 sandbox 和 AI agent 在同一主机的两个独立进程中，生命周期管理靠命令行，没有进程间健康检查，重启后需手动处理。仅适合本地开发调试。
:::

### 步骤

```bash
# 终端 1：启动 sandbox
lattice sandbox start \
  --name agent-001 \
  --server-url http://localhost:8080 \
  --token lt-xxx \
  --proxy-addr 127.0.0.1:1080 \
  --forward 8080:127.0.0.1:8080 \
  --egress-allow 10.0.0.0/8
```

```
# 终端 2：在 sandbox 所在的同一台机器上，启动 AI agent
export ALL_PROXY=socks5://127.0.0.1:1080
python my_ai_agent.py
```

### 缺点

- 两个进程没有绑定关系，sandbox 挂了 AI agent 不知道（流量静默走主机网络）
- 凭据持久化路径 `/etc/lattice/` 需要 root 创建，裸机用户通常是普通用户
- 不能利用 K8s/Docker 的服务发现和健康检查

---

## Mode 2: Docker Compose（推荐开发/单机）

Sandbox 和 AI agent 放在同一个 Compose stack 里，通过 `network_mode: "service:lattice-sandbox"` 让 AI agent 共享 sandbox 的网络栈，或者直接用 `localhost` 访问 sandbox 的 SOCKS5 端口。

### 单 service 模式（sandbox 作为容器入口）

当 AI agent 逻辑在容器镜像内、与 sandbox 同进程启动时：

```yaml
# docker-compose.yml
services:
  sandboxed-agent:
    image: my-ai-agent:latest
    command: ["sleep", "infinity"]   # 占位，实际逻辑在 sandbox 启动后执行
    environment:
      ALL_PROXY: socks5://localhost:1080
    volumes:
      - lattice-creds:/etc/lattice
    depends_on:
      lattice-sandbox:
        condition: service_healthy

  lattice-sandbox:
    image: ghcr.io/alatticeio/lattice:latest
    command:
      - sandbox
      - start
      - --name=agent-001
      - --server-url=http://latticed:8080
      - --token=${LATTICE_TOKEN}
      - --proxy-addr=127.0.0.1:1080
      - --forward=8080:127.0.0.1:8080
      - --egress-allow=10.0.0.0/8
      - --egress-default-deny
    volumes:
      - lattice-creds:/etc/lattice
    healthcheck:
      test: ["CMD", "curl", "-s", "--socks5", "localhost:1080", "http://latticed:8080/healthz"]
      interval: 10s
      timeout: 3s
      retries: 5

volumes:
  lattice-creds:
```

### 多 agent 场景（一 sandbox 对应一 agent）

```yaml
# docker-compose.multi.yml
services:
  sandbox-merlin:
    image: ghcr.io/alatticeio/lattice:latest
    command:
      - sandbox
      - start
      - --name=agent-merlin
      - --server-url=http://latticed:8080
      - --token=${LATTICE_TOKEN}
      - --proxy-addr=0.0.0.0:1080    # 允许多个 agent 容器访问

  agent-merlin:
    image: my-ai-agent:merlin
    environment:
      ALL_PROXY: socks5://sandbox-merlin:1080
    depends_on: [sandbox-merlin]

  sandbox-gandalf:
    image: ghcr.io/alatticeio/lattice:latest
    command:
      - sandbox
      - start
      - --name=agent-gandalf
      - --server-url=http://latticed:8080
      - --token=${LATTICE_TOKEN}
      - --proxy-addr=0.0.0.0:1080

  agent-gandalf:
    image: my-ai-agent:gandalf
    environment:
      ALL_PROXY: socks5://sandbox-gandalf:1080
    depends_on: [sandbox-gandalf]
```

### 验证

```bash
# 从 AI agent 容器内验证
docker compose exec agent-merlin curl --socks5 sandbox-merlin:1080 http://another-agent.lattice:8080
```

---

## Mode 3: Kubernetes Sidecar（推荐生产）

这是最自然的部署方式——sandbox 作为 Pod 中的 sidecar 容器，与 AI agent 容器共享 Pod 网络命名空间。从 Istio / Envoy 服务网格迁移过来的用户对这种模式最为熟悉。

### 基本 sidecar Pod

```yaml
# sandbox-sidecar-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: agent-001
  labels:
    lattice.io/agent: "true"
spec:
  containers:
    # ── AI Agent 容器 ──────────────────────────
    - name: agent
      image: my-ai-agent:latest
      env:
        - name: ALL_PROXY
          value: socks5://localhost:1080
        - name: NO_PROXY
          value: localhost,127.0.0.1,.local     # 避免循环代理
      # AI agent 只通过 SOCKS5 访问外部，不走 Pod 的 CNI

    # ── Sandbox sidecar ────────────────────────
    - name: lattice-sandbox
      image: ghcr.io/alatticeio/lattice:pro-latest
      command: ["lattice"]
      args:
        - sandbox
        - start
        - --name=$(AGENT_NAME)
        - --server-url=http://latticed.lattice-system:8080
        - --token=$(LATTICE_TOKEN)
        - --proxy-addr=127.0.0.1:1080
        - --forward=8080:127.0.0.1:8080
        - --egress-allow=10.0.0.0/8
      env:
        - name: AGENT_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: LATTICE_TOKEN
          valueFrom:
            secretKeyRef:
              name: lattice-enrollment
              key: token
      volumeMounts:
        - name: lattice-creds
          mountPath: /etc/lattice
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
      resources:
        requests:
          memory: "64Mi"
          cpu: "100m"
        limits:
          memory: "128Mi"
          cpu: "200m"

  volumes:
    - name: lattice-creds
      emptyDir: {}

---
# lattice-enrollment Secret
apiVersion: v1
kind: Secret
metadata:
  name: lattice-enrollment
type: Opaque
stringData:
  token: lt-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### Deployment 模式

```yaml
# sandbox-sidecar-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent-pool
spec:
  replicas: 3
  selector:
    matchLabels:
      app: agent-pool
  template:
    metadata:
      labels:
        app: agent-pool
        lattice.io/agent: "true"
      annotations:
        # 确保 agent 容器在 sandbox 就绪后才启动
        sidecar.lattice.io/socks5-ready: "true"
    spec:
      containers:
        - name: agent
          image: my-ai-agent:latest
          env:
            - name: ALL_PROXY
              value: socks5://localhost:1080
            - name: NO_PROXY
              value: localhost,127.0.0.1,.local
          lifecycle:
            postStart:
              exec:
                command:
                  - /bin/sh
                  - -c
                  # 等待 sandbox SOCKS5 端口可连接
                  - |
                    for i in $(seq 1 30); do
                      curl -s --socks5 localhost:1080 --max-time 2 http://latticed:8080/healthz && break
                      sleep 1
                    done

        - name: lattice-sandbox
          image: ghcr.io/alatticeio/lattice:pro-latest
          command: ["lattice"]
          args:
            - sandbox
            - start
            - --name=$(AGENT_NAME)
            - --server-url=http://latticed.lattice-system:8080
            - --token=$(LATTICE_TOKEN)
            - --proxy-addr=127.0.0.1:1080
            - --forward=8080:127.0.0.1:8080
            - --egress-allow=10.0.0.0/8
            - --egress-default-deny
          env:
            - name: AGENT_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: LATTICE_TOKEN
              valueFrom:
                secretKeyRef:
                  name: lattice-enrollment
                  key: token
          volumeMounts:
            - name: lattice-creds
              mountPath: /etc/lattice
          securityContext:
            runAsNonRoot: true
            runAsUser: 1000
          livenessProbe:
            exec:
              command:
                - curl
                - -s
                - --socks5
                - localhost:1080
                - --max-time
                - "3"
                - http://latticed.lattice-system:8080/healthz
            initialDelaySeconds: 15
            periodSeconds: 10

      volumes:
        - name: lattice-creds
          emptyDir: {}
```

### 关键注意事项

| 事项 | 说明 |
|------|------|
| **Shared network namespace** | Sandbox 和 agent 在同一个 Pod 内，共享 `localhost`，agent 通过 `localhost:1080` 连接 SOCKS5 |
| **`NO_PROXY`** | 必须设置 `localhost,127.0.0.1,.local`，防止 agent 连接 sandbox 的健康检查端点时走代理 |
| **启动顺序** | agent 容器应该在 sandbox 的 SOCKS5 端口就绪之后才开始实际工作（用 `postStart` hook 或 init 容器） |
| **Credential volume** | 用 `emptyDir` 存 `/etc/lattice/sandbox-credentials.json`，Pod 重启时自动恢复，不需重新注册 |
| **Token Secret** | Enrollment token 是一次性的，用完后从 Secret 中删除或标记。凭据持久化后，后续重启走 resume 路径，不消耗 token |
| **资源限制** | Sandbox 非常轻量（`64Mi + 100m`），gVisor netstack 在用户态，不占用内核资源 |
| **审计日志** | 本地写入 `/tmp/lattice-audit-<name>.jsonl`，Pro 版支持 NATS 流式审计回传到控制平面 |
| **Egress 白名单** | `--egress-allow 10.0.0.0/8` 只允许访问内网 overlay IP 段；加 `--egress-allow 0.0.0.0/0:443` 允许 HTTPS 公网访问 |

---

## 完整数据流（以 K8s Sidecar 为例）

```
┌──────────────────────────────────────────────────────────────┐
│  Pod: agent-001                                              │
│                                                              │
│  ┌──────────────────────────┐  ┌──────────────────────────┐  │
│  │ Container: AI Agent       │  │ Container: lattice-sandbox│  │
│  │                          │  │                          │  │
│  │ ALL_PROXY=socks5://      │  │ lattice sandbox start    │  │
│  │   localhost:1080         │  │   --proxy-addr :1080     │  │
│  │                          │  │   --forward 8080:...     │  │
│  │ requests.get(            │  │   --egress-allow ...     │  │
│  │   "http://svc/data")     │  │                          │  │
│  │       │                  │  │       ▲                  │  │
│  │       │ TCP              │  │       │ SOCKS5            │  │
│  │       ▼                  │  │       │                  │  │
│  │  connect("localhost",    │  │  Socks5Server             │  │
│  │    1080)                 │  │       │                  │  │
│  │       │                  │  │       ▼                  │  │
│  │       └──────────────────┼──│──▶ EgressFilter.Allow()  │  │
│  │                          │  │       │                  │  │
│  │                          │  │       ▼                  │  │
│  │                          │  │  gVisor netstack         │  │
│  │                          │  │       │                  │  │
│  │                          │  │  AuditWriter.Write()     │  │
│  │                          │  │       │                  │  │
│  │                          │  │       ▼                  │  │
│  │                          │  │  wireguard-go            │  │
│  │                          │  │       │                  │  │
│  │                          │  │  UDP :51820 ────────────▶ overlay
│  └──────────────────────────┘  └──────────────────────────┘  │
│                                                              │
│  ── 入站 ──────────────────────────────────────────────────▶  │
│  overlay peer → UDP :51820 → wireguard-go → netstack        │
│    → ForwardListener(:8080) → TCP → agent container :8080   │
└──────────────────────────────────────────────────────────────┘
```

**出站**：`agent 容器 → SOCKS5 :1080 → 策略检查 → 审计 → netstack → WireGuard → overlay`
**入站**：`overlay peer → WireGuard → netstack → ForwardListener → agent 容器端口`

---

## 故障排查

### Sandbox 无法注册

```bash
# 检查 NATS 连通性
kubectl exec agent-001 -c lattice-sandbox -- \
  curl -s http://latticed.lattice-system:8080/api/v1/signaling

# 检查 token 是否过期/已用
kubectl exec agent-001 -c lattice-sandbox -- \
  cat /etc/lattice/sandbox-credentials.json
# 如已过期，删除 credentials 并用新 token 重建 Pod
```

### AI agent 流量没有走 sandbox

```bash
# 在 agent 容器内验证代理是否生效
kubectl exec agent-001 -c agent -- \
  curl -v --socks5 localhost:1080 http://10.42.0.5:8080
# 如果直连也能通，说明 agent 没有设 ALL_PROXY
kubectl exec agent-001 -c agent -- env | grep PROXY
```

### 审计日志查看

```bash
# 本地审计文件（社区版）
kubectl exec agent-001 -c lattice-sandbox -- \
  cat /tmp/lattice-audit-agent-001.jsonl

# Pro 版：通过 API 查询集中存储的审计事件
curl "http://latticed.lattice-system:8080/api/v1/audit/flow?agentName=agent-001"
```

---

## Mode 4: gVisor runsc 模式（Coming Soon）

> **状态：设计完成，待实现。** 详细设计见 [ADR 0002 — Sandbox Isolation Model](/adr/0002-sandbox-isolation-model)。

当前的 SOCKS5 sidecar 模式（Mode 2/3）提供的是**网络层隔离**——AI agent 通过代理接入 overlay。对于需要**不可绕过**的隔离场景——例如运行不受信任的第三方 agent 代码——Lattice 正在实现 **runsc（gVisor 容器运行时）模式**。

runsc 模式下，AI agent 进程运行在 gVisor sentry（用户态内核）内部，所有 syscall 被拦截。网络层通过 `--pass-fd` 传入的 Unix domain socket 接入 Lattice SOCKS5 代理，**所有 network stack 组件与 pod 模式完全复用**。

### 设计中的对比

| | SOCKS5 Sidecar（pod） | gVisor runsc |
|---|---|---|
| **隔离层级** | 网络层（代理自觉接入） | 进程级（syscall 强制拦截） |
| **AI agent 如何接入** | 设置 `ALL_PROXY` 环境变量 | `ALL_PROXY=socks5://unix:/lattice-socks` |
| **网络路径** | `agent → socks5(TCP) → netstack → WireGuard` | `agent → socks5(Unix) → netstack → WireGuard` |
| **能否绕过** | 可以（不设代理，直连） | 不可绕过（所有 syscall 被 gVisor 拦截，仅 loopback） |
| **DNS 泄漏** | 可能 | 不可能 |
| **特权要求** | 零特权 | 需安装 runsc 运行时 |
| **性能开销** | 接近零 | syscall 转译约 5–15% |
| **复用组件** | — | netstack、Socks5Server、EgressFilter、AuditWriter、ForwardListener、TUNAdapter 全部复用 |

### 设计 CLI

```bash
# runsc 模式启动（设计阶段，CLI 可能变化）
lattice sandbox start \
  --mode gvisor \
  --name agent-001 \
  --server-url http://localhost:8080 \
  --token lt-xxx \
  --proxy-addr 127.0.0.1:1080 \
  --agent-rootfs /opt/lattice/agent-rootfs \
  --agent-binary /usr/local/bin/ai-agent \
  --agent-args "--model,gpt-4,--verbose" \
  --forward 8080:127.0.0.1:8080 \
  --egress-allow 10.0.0.0/8 \
  --egress-default-deny
```

**新增 flags（设计）：**

| Flag | 说明 |
|------|------|
| `--mode` | 隔离模式：`pod`（默认）\| `gvisor` |
| `--agent-rootfs` | runsc 容器的根文件系统路径 |
| `--agent-binary` | agent 入口二进制路径 |
| `--agent-args` | agent 启动参数（逗号分隔） |

### 出站和入站的分工

两种模式下，出站（agent 主动发起）和入站（外部 peer 连入）都走不同的组件：

| 方向 | pod 模式路径 | runsc 模式路径 |
|------|------------|---------------|
| **出站** | `agent → host TCP :1080 → Socks5Server → gVisor netstack → WG` | `agent → fd 3 → socketpair → Socks5Server → gVisor netstack → WG` |
| **入站** | `WG → gVisor netstack → ForwardListener → host TCP → agent` | `WG → gVisor netstack → ForwardListener → socketpair → fd 3 → agent` |

- **Socks5Server** 处理出站：agent 发起 SOCKS5 CONNECT，代理代它建 gVisor TCP。响应沿同一条 SOCKS5 连接返回，不需要额外通路。
- **ForwardListener** 处理入站：在 gVisor netstack 上监听 overlay 端口，收到连接后转发给 agent。

两者不是上下游关系，是**并行的两条独立管线**，共用同一个 gVisor netstack。runsc 模式保留分工不变，只把 host TCP 接驳换成同一个 `socketpair`。

### 设计数据流

```
  pod 模式（当前）:
  ┌──────────────────────┐              ┌───────────────────────┐
  │ Socks5Server         │              │ ForwardListener        │
  │ accept(host TCP)     │              │ accept(gVisor TCP)     │
  │   ↓ SOCKS5 CONNECT   │              │   ↓                    │
  │ DialContext(gVisor)  │              │ net.Dial(host TCP)     │
  │ relay(host ↔ gVisor) │              │ relay(gVisor ↔ host)   │
  └──────────────────────┘              └───────────────────────┘

  runsc 模式（设计）:
  ┌──────────────────────┐              ┌───────────────────────┐
  │ Socks5Server         │              │ ForwardListener        │
  │ accept(socketpair)   │              │ accept(gVisor TCP)  ← 不变
  │   ↓ SOCKS5 CONNECT   │              │   ↓                    │
  │ DialContext(gVisor) ← 不变          │ write(socketpair)      │  ← 替代 net.Dial
  │ relay(sp ↔ gVisor)   │              │ relay(gVisor ↔ sp)     │
  └──────────────────────┘              └───────────────────────┘
```

**关键：** gVisor netstack、EgressFilter、AuditWriter、channel.Endpoint、TUNAdapter、wireguard-go 在两种模式下路径完全一致。唯一变化是 agent 侧的接驳从 host TCP 换成 socketpair。

### 安全边界

runsc 容器内的 AI agent：

- **syscall 强制拦截** — 所有 socket/connect 调用被 gVisor sentry 拦截，仅 loopback 可达
- **文件系统受限** — gofer 代理的只读 rootfs
- **无法接触密钥** — WireGuard 私钥、NATS JWT 仅存在于 host 进程
- **无法逃逸** — 无主机网络、无 `/dev/net/tun`、无 `CAP_NET_ADMIN`

### 设计集成方式

runsc 模式不依赖容器运行时（Docker/K8s）——`lattice sandbox start --mode gvisor` 直接启动 runsc 子进程。因此在 Docker Compose 或 K8s Sidecar 中部署时，sandbox 容器内跑的就是 `lattice sandbox start --mode gvisor`，由它拉起 runsc：

```yaml
# K8s Sidecar（runsc 模式）
- name: lattice-sandbox
  image: ghcr.io/alatticeio/lattice:pro-latest
  command: ["lattice"]
  args:
    - sandbox
    - start
    - --mode=gvisor
    - --name=$(AGENT_NAME)
    - --server-url=http://latticed:8080
    - --token=$(LATTICE_TOKEN)
    - --proxy-addr=127.0.0.1:1080
    - --agent-rootfs=/opt/lattice/agent-rootfs
    - --agent-binary=/usr/local/bin/ai-agent
    - --forward=8080:127.0.0.1:8080
    - --egress-allow=10.0.0.0/8
    - --egress-default-deny
  securityContext:
    privileged: true  # runsc 需要特权模式启动 sentry
  volumeMounts:
    - name: lattice-creds
      mountPath: /etc/lattice
```

### 设计迁移路径

从 SOCKS5 sidecar 迁移到 runsc 时，底层组件基本复用：

- `EgressFilter` — 复用，策略检查逻辑不变
- `AuditWriter` — 复用，审计记录逻辑不变
- `Socks5Server` — 复用，额外增加 Unix socket 监听（读 socketpair 处理 CONNECT）
- `ForwardListener` — 复用入口，但不再 `net.Dial` TCP target，而是往 socketpair host-end 写入站连接数据
- `TUNAdapter` / `wireguard-go` — 零改动

详见 [ADR 0002 — Sandbox Isolation Model](/adr/0002-sandbox-isolation-model)。

---

## STUN 服务

当前 ICE NAT 穿透使用自研 `lattice start turn` 部署的 STUN 服务。设计上计划用 coturn STUN-only 统一公网服务替换，详见 [STUN 部署设计](../superpowers/specs/2026-05-19-stun-deployment-design)。
