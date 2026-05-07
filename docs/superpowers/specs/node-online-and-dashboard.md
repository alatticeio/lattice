# 节点在线 & Dashboard 配置指南

## 一、节点在线

### 整体流程

```
管理员                         控制面 (latticed)               节点 (wireflow)
  |                                  |                               |
  |-- 创建工作空间 -----------------> |                               |
  |-- 生成 Token -----------------> |                               |
  |                                  |                               |
  |                                  | <-- wireflow up --token ... --|
  |                                  |  (NATS: peer.register)        |
  |                                  |-- 分配 IP, 写入 WireGuard --->|
  |                                  |                               |
  |                                  | <-- heartbeat (每 30s) -------|
  |                                  |  在线状态: lastSeen < 90s     |
```

### 第一步：控制面必须具备

控制面 `latticed` 启动时需要以下三个连接：

| 配置项 | 说明 | 示例 |
|--------|------|------|
| `signaling-url` | NATS 信令服务地址，节点通过此地址注册和心跳 | `nats://localhost:4222` |
| `server-url` | 管理 API 地址（节点自注册时用） | `http://localhost:8080` |
| `database.dsn` | 数据库（空 = 自动使用本地 SQLite） | 留空即可 |

`wireflow.yaml` 最小配置：

```yaml
signaling-url: "nats://localhost:4222"
listen: ":8080"
app:
  initAdmins:
    - username: admin
      password: your-password
```

K8s 部署（all-in-one）时，NATS 和 Manager 地址通过 Service 环境变量自动注入，无需手动填写。

---

### 第二步：在管理界面准备好工作空间和 Token

1. 登录管理界面
2. **工作空间** → 创建一个工作空间（会自动创建对应的 K8s Namespace 和 WireflowNetwork）
3. **节点** → 点击「生成入网令牌」→ 复制 Token

---

### 第三步：节点启动

在目标机器上执行 `wireflow up`：

```bash
# 最简启动（必填三项）
wireflow up \
  --token    <从管理界面复制的 Token> \
  --server-url   http://<控制面地址>:8080 \
  --signaling-url nats://<控制面地址>:4222

# 保存配置，下次直接 wireflow up 即可
wireflow up \
  --token <token> \
  --server-url http://... \
  --signaling-url nats://... \
  --save
```

保存后配置写入 `~/.wireflow/wireflow.yaml`，之后只需运行 `wireflow up`。

#### 完整参数说明

| 参数 | 必填 | 说明 |
|------|------|------|
| `--token` | 是 | 入网令牌（管理界面生成） |
| `--server-url` | 是 | 控制面 HTTP API 地址 |
| `--signaling-url` | 是 | NATS 信令地址 |
| `--wg-port` | 否 | WireGuard/ICE UDP 端口，默认 `51820` |
| `--enable-wrrp` | 否 | 开启中继穿透（严苛 NAT 环境） |
| `--wrrper-url` | 否 | 中继服务器 TCP 地址，配合 `--enable-wrrp` |
| `--wrrp-quic-url` | 否 | 中继服务器 QUIC 地址（优先级高于 TCP） |
| `--save` | 否 | 将以上参数持久化到配置文件 |

---

### 第四步：确认节点在线

节点启动后通过 NATS 信令自动注册，之后每 **30 秒**发送一次心跳。

管理界面「节点」列表在线状态判定规则：

| 状态 | 含义 |
|------|------|
| `online`  | 最近 90 秒内收到心跳 |
| `offline` | 曾经注册，但超过 90 秒未收到心跳 |
| `pending` | 已注册，但从未收到过心跳（节点还未真正启动连接） |

---

### NAT 穿透（可选）

如果节点处于严苛 NAT 后面（P2P 直连失败），需要配置中继：

1. 管理界面「设置 → 中继服务器」中添加一台中继服务器（填写 TCP/QUIC 地址）
2. 节点启动时加上：

```bash
wireflow up \
  ... \
  --enable-wrrp \
  --wrrper-url relay.example.com:6266 \
  --wrrp-quic-url relay.example.com:6267   # 可选，QUIC 优先
```

---

## 二、Dashboard 数据显示

Dashboard 的流量趋势、吞吐量等数据来自 **VictoriaMetrics**（兼容 Prometheus PromQL API）。需要两端都配置。

### 架构

```
节点 (wireflow)
  |-- 每 30s 推送指标 --> VictoriaMetrics (/api/v1/write)
                               |
控制面 (latticed)             |
  |-- PromQL 查询 ----------> VictoriaMetrics (:8428)
  |-- 返回 Dashboard 数据 --> 前端
```

---

### 控制面侧配置

在 `wireflow.yaml` 中添加 `monitor.address`，指向 VictoriaMetrics 的 HTTP 地址：

```yaml
monitor:
  address: "http://localhost:8428"   # 或 http://victoria-metrics-svc:8428
```

K8s 部署时可用环境变量：

```yaml
env:
  - name: WIREFLOW_MONITOR_ADDRESS
    value: "http://victoria-metrics:8428"
```

**未配置此项时**，Dashboard 页面的统计卡片、流量趋势图均无数据显示（显示 `—` 或空图）。

---

### 节点侧配置

节点需要开启指标推送，将 WireGuard 流量数据上报到 VictoriaMetrics：

```bash
wireflow up \
  ... \
  --enable-metric \
  --vm-endpoint http://<VictoriaMetrics地址>:8428/api/v1/write
```

或写入配置文件 `~/.wireflow/wireflow.yaml`：

```yaml
enable-metric: true
telemetry:
  vmEndpoint: "http://victoria-metrics:8428/api/v1/write"
  intervalSeconds: 30   # 推送间隔，默认 30s
```

**未配置此项时**，控制面查到的所有流量数据为 0，节点在 Dashboard 上显示为无流量。

---

### 部署 VictoriaMetrics（单机最简）

```bash
docker run -d \
  --name victoria-metrics \
  -p 8428:8428 \
  victoriametrics/victoria-metrics:latest
```

K8s 可以用官方 Helm chart：

```bash
helm repo add vm https://victoriametrics.github.io/helm-charts
helm install victoria-metrics vm/victoria-metrics-single \
  --set server.persistentVolume.enabled=true
```

---

### Dashboard 各模块数据来源汇总

| 模块 | 数据来源 | 依赖 |
|------|----------|------|
| 节点在线状态 | NATS 心跳（内存） | 只需节点正常连接 |
| 节点列表 / IP | K8s WireflowPeer CRD | 只需控制面正常 |
| 工作空间流量趋势 | VictoriaMetrics PromQL | 节点 + 控制面均配置 VM |
| 吞吐量统计卡片 | VictoriaMetrics PromQL | 同上 |
| 拓扑图 | WireflowPeer + WireflowNetwork CRD | 只需控制面正常 |

---

## 三、最简验证清单

```
[ ] latticed 已启动，signaling-url / listen 已配置
[ ] NATS 服务可达（latticed 内置或独立部署）
[ ] 管理界面可登录，工作空间已创建
[ ] Token 已生成
[ ] 节点执行 wireflow up --token ... --server-url ... --signaling-url ...
[ ] 管理界面「节点」列表出现该节点，状态变为 online
[ ] (可选) VictoriaMetrics 已部署
[ ] (可选) 控制面 monitor.address 已配置
[ ] (可选) 节点 --enable-metric --vm-endpoint 已配置
[ ] (可选) Dashboard 流量图有数据
```
