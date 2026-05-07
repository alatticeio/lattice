# Performance Benchmark Design

We measure three dimensions — Time-to-First-Handshake (TTFH), WireGuard tunnel throughput, and control-plane API p99 latency — using agent-side instrumentation rather than external polling, and publish results as Shields.io badges backed by JSON files on the `gh-pages` branch.

## Considered Options

**Agent-side instrumentation (chosen):** The agent records `t0` at process start and emits `lattice_peer_handshake_duration_seconds` when `wgtypes.Peer.LastHandshakeTime` transitions from zero to non-zero. Results are pushed to VictoriaMetrics and also written to `gh-pages` JSON by CI.

**External observer polling (rejected):** The test harness polls `wg show` in a loop and measures wall-clock time. Simpler to implement but includes container startup overhead (±500ms noise) and cannot be observed in production deployments.

## Consequences

- `lattice_peer_handshake_duration_seconds` is a **Community-visible metric** — it must never be moved behind the Pro feature gate, as it is the primary signal that an installation is working correctly.
- TTFH SLA tiers are: LAN ≤ 3 s, NAT ≤ 8 s, Relay ≤ 15 s.
- CI benchmark runs weekly (`schedule`) plus on `workflow_dispatch`. A **Benchmark SLA Gate** (hard CI failure) is deliberately deferred until 4 weeks of baseline data are collected on shared GitHub Actions runners to avoid false positives from network jitter.
- Benchmark results are committed to `gh-pages` as Shields.io endpoint JSON; README badges update automatically after each run.
