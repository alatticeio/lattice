# Lattice 性能测试设计

> 2026-05-21 | 设计阶段

## 目标

为 Lattice 建立分层性能测试体系，产出可放入 README 的 benchmark 数据和曲线图，向用户展示 overlay 网络的性能损耗。

---

## 分层策略

### 第一层：组件级 Benchmark（CI 自动化）

Go 内置 `testing.B`，每次 push 自动跑，结果生成历史曲线图。

| Benchmark | 测试内容 | 指标 | 目标值 |
|-----------|---------|------|--------|
| `BenchmarkWireGuardEncrypt` | WireGuard ChaCha20-Poly1305 加密 1 packet | ns/op, B/op | < 15 μs |
| `BenchmarkFilteringUDPMux` | STUN/WG 包解复用（magic cookie 检测） | ns/op | < 0.5 μs |
| `BenchmarkLRPFrameEncode` | LRP 12-byte header 编码 + 解密 | ns/op | < 5 μs |
| `BenchmarkEgressFilterCheck` | CIDR allowlist 匹配（10 条规则） | ns/op | < 1 μs |
| `BenchmarkSandboxProvisioner` | 100 peer 的 WireGuard 配置生成 | ms/op | < 10 ms |

### 第二层：集成级 Benchmark（CI 自动化）

单机 Docker 环境，测组件间协作。

| Benchmark | 测试内容 | 环境 |
|-----------|---------|------|
| `BenchmarkICEDialLocal` | 同机 ICE 握手耗时（无 NAT） | Docker 内两个进程 |
| `BenchmarkNatSDial` | NATS 信令消息往返延迟 | Docker latticed |
| `BenchmarkSandboxBootstrap` | sandbox pod 模式启动耗时 | Docker + lattice CLI |

### 第三层：端到端 Benchmark（云主机手动跑）

3 台云主机（北京 + 北京 + 上海），真实跨公网场景。

| 场景 | 工具 | 指标 |
|------|------|------|
| **吞吐量** | `iperf3` | overlay 带宽 vs 裸金属 |
| **延迟** | `ping` | overlay RTT vs 直连 |
| **ICE 握手** | lattice 日志计时 | SYN→Connected 耗时 |
| **LRP relay 回退** | 防火墙阻断 UDP，强制走 relay | relay 吞吐 vs 直连 |
| **Sandbox 启动** | `lattice sandbox start` 计时 | 注册 + wg0 创建耗时 |

---

## 工具链

```
┌─────────────────────────────────────────────────────────┐
│ Go benchmark          外部工具 (iperf3, ping)            │
│ (go test -bench)                                       │
│      │                      │                          │
│      ▼                      ▼                          │
│ benchstat               JSON 汇总脚本                   │
│ (统计对比)              (shell + jq)                    │
│      │                      │                          │
│      └──────────┬───────────┘                          │
│                 ▼                                      │
│         github-action-benchmark                        │
│         (CI 自动生成历史曲线)                           │
│                 │                                      │
│                 ▼                                      │
│          README.md 嵌入                                │
│          ![benchmark](bench/throughput.svg)            │
└─────────────────────────────────────────────────────────┘
```

### 依赖

```bash
# 安装
go install golang.org/x/perf/cmd/benchstat@latest
pip install matplotlib       # 手动制图时用
apt install iperf3           # 端到端吞吐测试
```

---

## 目录结构

```
bench/
├── go/
│   ├── wg_encrypt_test.go       # BenchmarkWireGuardEncrypt
│   ├── mux_filter_test.go       # BenchmarkFilteringUDPMux
│   ├── lrp_frame_test.go        # BenchmarkLRPFrameEncode
│   ├── egress_filter_test.go    # BenchmarkEgressFilterCheck
│   └── sandbox_provisioner_test.go
├── docker/
│   ├── ice_dial_test.go         # BenchmarkICEDialLocal
│   └── nats_test.go             # BenchmarkNatSDial
├── e2e/
│   ├── throughput.sh            # iperf3 overlay vs bare
│   ├── latency.sh               # ping RTT comparison
│   └── ice_handshake.sh         # ICE timing analysis
├── results/
│   └── .gitkeep                 # 历史数据 JSON
└── scripts/
    ├── run_all.sh               # 一键跑所有 benchmark
    └── plot.py                  # 手动生成图表
```

---

## CI 集成

```yaml
# .github/workflows/benchmark.yml
name: Benchmark

on:
  push:
    branches: [dev, master]

jobs:
  component:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - name: Run component benchmarks
        run: go test -bench=. -benchmem -count=5 ./bench/go/... > bench.txt
      - name: Store result
        uses: benchmark-action/github-action-benchmark@v1
        with:
          tool: go
          output-file-path: bench.txt
          github-token: ${{ secrets.GITHUB_TOKEN }}
          auto-push: true
```

---

## README 展示方案

组件级用 CI 自动生成的曲线图，端到端用手动跑的表格：

```markdown
## Performance

### Throughput (iperf3, cross-region)

| Scenario | Bare Metal | WireGuard Overlay | Overhead |
|----------|-----------|-------------------|----------|
| TCP (Beijing → Shanghai) | 940 Mbps | 890 Mbps | **5.3%** |
| UDP (Beijing → Shanghai) | 950 Mbps | 870 Mbps | **8.4%** |

### Latency

| Scenario | Direct | Overlay | Delta |
|----------|--------|---------|-------|
| ping RTT (same region) | 1.2 ms | 2.8 ms | +1.6 ms |
| ping RTT (cross region) | 28 ms | 31 ms | +3 ms |

### Handshake

| Phase | Time |
|-------|------|
| ICE SYN → Connected (no NAT) | 2.1 s |
| ICE SYN → Connected (Cone NAT) | 3.2 s |
| LRP relay fallback | 1.8 s |

### Component Benchmarks

![WireGuard Encrypt](bench/wg_encrypt.svg)
![FilteringUDPMux](bench/mux_filter.svg)
```

---

## 待定

- [ ] 组件级 benchmark 先做哪几个？（建议先做 WireGuard 加密 + Mux filter，这两个最直接体现核心性能）
- [ ] 端到端测试需要等你 3 台云主机就绪后再跑
- [ ] CI benchmark action 需要 repo 的 `GITHUB_TOKEN` secrets 权限
