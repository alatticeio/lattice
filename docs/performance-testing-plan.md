# Performance Testing Plan

## Background

Lattice is an overlay networking product. Performance is a first-class concern: users evaluate it against Tailscale and Netbird, where the bar is sub-3s connection time on LAN and multi-hundred-Mbps tunnel throughput. This document describes the three measurement dimensions, their SLAs, how they are instrumented, and how results are published.

## Dimensions and SLAs

### 1. Time-to-First-Handshake (TTFH)

**Definition:** Elapsed time from `lattice up` process start to the first successful WireGuard handshake with any peer (`wgtypes.Peer.LastHandshakeTime` transitions from zero to non-zero).

**Why it matters:** This is the latency a user perceives when connecting to the mesh. It exercises the full stack: peer registration → NATS signaling → ICE negotiation → WireGuard handshake.

**SLA tiers (informed by Tailscale / Netbird benchmarks):**

| Scenario | Description | SLA |
|----------|-------------|-----|
| LAN | Both peers on same Docker bridge or L2 segment | ≤ 3 s |
| NAT | Peers behind consumer-grade NAT, ICE STUN hole-punch | ≤ 8 s |
| Relay | Hard NAT / symmetric NAT, LRP Relay fallback | ≤ 15 s |

**Instrumentation (agent-side):**
- `t0`: recorded at `agent.Start()` entry
- `t1`: recorded when WireGuard status loop first detects `LastHandshakeTime != zero`
- Metric: `lattice_peer_handshake_duration_seconds{scenario, peer_id}`
- Availability: **Community + Pro** (this is a health indicator, not a Pro feature)
- Code location: `internal/agent/wireguard/status.go` (already reads `LastHandshakeTime`)

**CI:** Measured in `bench.yml` using `hack/bench/handshake_bench.sh` (3 runs, LAN scenario, Docker containers). SLA gate deferred until 4 weeks of baseline data accumulated.

---

### 2. WireGuard Tunnel Throughput

**Definition:** Maximum sustained TCP throughput through the WireGuard overlay (`wf0` interface), measured with iperf3 between two connected peers.

**Why it matters:** Users running file sync, backup, or video streams over Lattice need to know the overhead vs. raw network speed.

**SLA:**

| Condition | Target |
|-----------|--------|
| LAN, single stream | ≥ 800 Mbps |

**Benchmark:** iperf3 server on peer B, client on peer A, 10-second run, JSON output parsed for `bits_per_second`.

**Reference:** WireGuard raw throughput on modern hardware is ~950 Mbps at 1 Gbps NIC. The 800 Mbps target (~85%) accounts for WireGuard crypto overhead.

**CI:** `hack/bench/throughput_bench.sh`, runs after handshake_bench (reuses running containers).

---

### 3. Control-Plane API p99 Latency

**Definition:** 99th-percentile response time for `GET /api/v1/workspaces` under 10 concurrent clients, measured with `hey`.

**Why it matters:** Slow control-plane responses affect the Dashboard UX and CLI responsiveness. It also surfaces database query regressions (SQLite under concurrent reads).

**SLA:**

| Condition | Target |
|-----------|--------|
| Single-node SQLite, 10c / 200 requests | p99 ≤ 50 ms |

**CI:** `hack/bench/api_bench.sh`.

---

## Instrumentation Roadmap

### Phase 1 — CI benchmark scripts (done)
- `hack/bench/handshake_bench.sh`
- `hack/bench/throughput_bench.sh`
- `hack/bench/api_bench.sh`
- `.github/workflows/bench.yml` (weekly schedule + workflow_dispatch)

### Phase 2 — Agent-side metric (next)
Add `lattice_peer_handshake_duration_seconds` to the agent:

```go
// internal/agent/node.go — record t0
var agentStartTime = time.Now()

// internal/agent/wireguard/collector.go — detect t1, emit metric
if !peer.LastHandshakeTime.IsZero() && !p.handshakeRecorded {
    duration := peer.LastHandshakeTime.Sub(agentStartTime).Seconds()
    handshakeDuration.WithLabelValues(peer.PublicKey.String()).Observe(duration)
    p.handshakeRecorded = true
}
```

This metric is available in Community builds. Pro builds push it to VictoriaMetrics via the existing telemetry pipeline.

### Phase 3 — SLA Gate (deferred, ~4 weeks after Phase 1)
After collecting baseline data from CI runs, add a hard assertion to `bench.yml`:
```bash
[ "$avg_ms" -le 3000 ] || { echo "FAIL: TTFH ${avg_s}s > 3s SLA"; exit 1; }
```

### Phase 4 — NAT / Relay scenarios (manual, not CI)
Cross-NAT and relay scenarios cannot be reliably automated on shared GitHub Actions runners due to network topology constraints. These are measured manually using `hack/bench/handshake_bench.sh` with the `RUNS` variable, against a real deployment.

---

## Badge Display

Results are committed to the `gh-pages` branch as Shields.io endpoint JSON after each CI run. Add to README.md:

```markdown
[![Handshake LAN](https://img.shields.io/endpoint?url=https://alatticeio.github.io/lattice/docs/benchmarks/handshake.json)](https://github.com/alatticeio/lattice/actions/workflows/bench.yml)
[![Throughput](https://img.shields.io/endpoint?url=https://alatticeio.github.io/lattice/docs/benchmarks/throughput.json)](https://github.com/alatticeio/lattice/actions/workflows/bench.yml)
[![API p99](https://img.shields.io/endpoint?url=https://alatticeio.github.io/lattice/docs/benchmarks/api-p99.json)](https://github.com/alatticeio/lattice/actions/workflows/bench.yml)
```

Badge color thresholds:

| Metric | brightgreen | green | yellow | red |
|--------|-------------|-------|--------|-----|
| TTFH | ≤ 2s | ≤ 3s | ≤ 5s | > 5s |
| Throughput | ≥ 900 Mbps | ≥ 800 Mbps | ≥ 600 Mbps | < 600 Mbps |
| API p99 | ≤ 20ms | ≤ 50ms | ≤ 100ms | > 100ms |

---

## Competitive Context

| Product | LAN TTFH | Throughput |
|---------|----------|-----------|
| Tailscale | 1–3 s | ~950 Mbps |
| Netbird | 2–5 s | ~900 Mbps |
| ZeroTier | 5–15 s | ~700 Mbps |
| **Lattice target** | **≤ 3 s** | **≥ 800 Mbps** |

The WireGuard handshake itself is < 500ms. TTFH budget is dominated by signaling (NATS round-trips + ICE candidate exchange). Reducing TTFH below 2s requires minimising signaling latency — a future optimisation once baseline is established.
