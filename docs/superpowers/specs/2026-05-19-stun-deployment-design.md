# STUN 部署方案 — 用 coturn 替换自研 STUN，统一公网服务

> 生成日期: 2026-05-19

## 背景

当前 Lattice 的 STUN 服务通过 `lattice start turn` 命令启动（`pion/turn` 内嵌），部署在 `config/lattice/base/deployment.yaml` 中的 `turn` Deployment 里。ICE 拨号器（`internal/server/transport/ice_dialer.go`）硬编码 `stun.alattice.io:3478` 作为 STUN 服务器地址。

问题：
- `lattice start turn` 内嵌 TURN 功能过剩（ICE 只用 STUN binding，`WithCandidateTypes` 只收集 Host + ServerReflexive，不用 relay candidate）
- STUN 地址硬编码在 `ice_dialer.go` 中，改地址需重新编译
- 自研维护成本：需要自己编译、打包、部署、随 Lattice 发版升级

## 方案：coturn STUN-only 统一公网服务

用 coturn 替换 `lattice start turn`，作为 Lattice 统一公网 STUN 服务（`stun.alattice.io`），所有 agent 共用。

```
                   ┌──────────────────────┐
                   │  stun.alattice.io    │
                   │  (coturn STUN-only)  │
                   │  UDP :3478           │
                   └──────────┬───────────┘
                              │
      ┌───────────────────────┼───────────────────────┐
      │                       │                       │
┌─────▼─────┐          ┌─────▼─────┐          ┌─────▼─────┐
│ 集群 A     │          │ 集群 B     │          │ 裸机 Agent │
└───────────┘          └───────────┘          └───────────┘
```

**对 all-in-one 部署的影响**：删掉 `turn` Deployment，不新增 Pod。STUN 作为外部依赖（和 NATS/DB 同级），不在集群内跑。

## 安全考虑

STUN 是无认证、无状态的 UDP 协议，主要威胁：

| 威胁 | 说明 | 严重度 |
|------|------|--------|
| DDoS 反射放大 | 伪造源 IP 发 STUN request，response 打到受害者 | 低 — 放大系数仅 ~1.5x（20→32 bytes），远低于 DNS（~50x） |
| 资源耗尽 | 大量 binding request 占满 CPU/UDP 端口 | 中 |
| 信息泄露 | 探测服务存活状态 | 低 |

### 缓解措施

1. **用户基数可控** — `stun.alattice.io` 只有 Lattice agent 使用，不是公共 STUN，流量天然有限
2. **iptables rate limit**（最有效）：

   ```bash
   iptables -A INPUT -p udp --dport 3478 \
     -m limit --limit 50/sec --limit-burst 100 -j ACCEPT
   iptables -A INPUT -p udp --dport 3478 -j DROP
   ```

3. **无状态，易扩容** — 如果流量增长，DNS round-robin 指向多个 coturn 实例即可，不需要会话同步

## 部署变更

### 新增 coturn Deployment

```yaml
# 跑在公网 VPS 上，或和 latticed 同机
apiVersion: apps/v1
kind: Deployment
metadata:
  name: stun
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: coturn
          image: coturn/coturn:4.6
          args:
            - "--stun-only"
            - "--port=3478"
          ports:
            - containerPort: 3478
              protocol: UDP
              hostPort: 3478  # 必须 hostPort/hostNetwork，否则返回 Pod IP
```

### 删除原 turn Deployment

`config/lattice/base/deployment.yaml` 中删除 `turn` Deployment。

### 代码改动

`ice_dialer.go` 读 config 替代硬编码：

```go
// before: 硬编码
{Scheme: stun.SchemeTypeSTUN, Host: "stun.alattice.io", Port: 3478}

// after: 读 config.stun-url，默认值不变
host, portStr, _ := net.SplitHostPort(agentconfig.Conf.TurnServerURL)
port, _ := strconv.Atoi(portStr)
{Scheme: stun.SchemeTypeSTUN, Host: host, Port: port}
```

config 中已有 `stun-url` 字段（默认 `stun.alattice.io:3478`），ICE dialer 之前没用它。

## 影响

- `stun.alattice.io` 成为 Lattice 的关键基础设施依赖。挂了会降低 ICE NAT 穿透成功率，但 ICE 会回退到 Host candidate 或 LRP relay，不会完全断连
- coturn 需要基础监控（CPU、UDP 端口可达性、packet rate）
- 后续可轻松扩展为多实例（DNS round-robin 或 anycast）
