# Lattice

Lattice is an open-core overlay networking product built on WireGuard + ICE. It runs as a K8s operator (control plane) paired with a lightweight edge agent, and exposes a mesh network between enrolled nodes. Community edition is Apache 2.0; Pro edition adds monitoring, SSO, eBPF enforcement, and telemetry.

## Language

### Core networking

**Peer**:
A node enrolled in a Lattice workspace, represented as a `LatticePeer` CRD. Has a WireGuard public key and an allocated overlay IP.
_Avoid_: node, agent, client

**Workspace**:
An isolated network segment, implemented as a K8s namespace. Peers within a workspace share a WireGuard mesh.
_Avoid_: network, tenant, project

**Enrollment Token**:
A time-limited credential that authorises a Peer to join a specific Workspace.
_Avoid_: join token, invite, access key

**Overlay IP**:
The WireGuard IP address allocated to a Peer within a Workspace (e.g. `10.96.0.4/32`).
_Avoid_: VPN IP, tunnel IP, WG address

**Network Policy**:
A `LatticePolicy` CRD that allows or denies traffic between Peers, enforced by iptables (Community) or eBPF TC (Pro).
_Avoid_: firewall rule, ACL

### Transport

**ICE Path**:
Direct peer-to-peer UDP path negotiated via Interactive Connectivity Establishment. Preferred over relay when reachable.
_Avoid_: direct path, p2p

**LRP Relay**:
The Lattice Relay Protocol fallback transport, used when ICE cannot establish a direct path (hard NAT / firewall). TCP and QUIC variants exist.
_Avoid_: TURN, relay, fallback

**Signaling**:
The NATS-based channel used to exchange ICE candidates and peer configuration between control plane and agents. URL is auto-discovered via `GET /api/v1/discovery`.
_Avoid_: NATS channel, pub/sub

### Connection lifecycle

**Time-to-First-Handshake (TTFH)**:
The elapsed time from agent process start (`lattice up`) to the first successful WireGuard handshake with any peer. The primary end-user latency SLA metric.
_Avoid_: connection time, startup time, join time

**Handshake Duration**:
Synonym for TTFH when used in metrics (`lattice_peer_handshake_duration_seconds`). Recorded agent-side at the moment `wgtypes.Peer.LastHandshakeTime` transitions from zero to non-zero.

**Connection Scenario**:
The network topology class under which TTFH is measured. Three tiers:
- **LAN**: both peers on the same L2 segment or inside Docker bridge — ICE resolves host candidates directly.
- **NAT**: peers behind consumer-grade NAT — ICE performs STUN hole-punching.
- **Relay**: hard NAT or symmetric NAT where ICE fails — traffic falls back to LRP Relay.

**TTFH SLA**:
The per-scenario upper bound for Time-to-First-Handshake:
- LAN ≤ 3 s
- NAT ≤ 8 s
- Relay ≤ 15 s

### Performance benchmarking

**Benchmark Suite**:
The set of three automated scripts in `hack/bench/` that measure TTFH, WireGuard tunnel throughput, and control-plane API latency. Run weekly by CI and on `workflow_dispatch`.

**Benchmark Badge**:
A Shields.io dynamic badge in README.md backed by a JSON endpoint file committed to the `gh-pages` branch. Updated by the Benchmark Suite after each run.

**Benchmark SLA Gate**:
A CI assertion that fails the build if a benchmark result exceeds its SLA threshold. Intentionally deferred until 4 weeks of baseline data are collected to avoid false positives on shared GitHub Actions runners.

### Editions

**Community Edition**:
The default build (no `-tags pro`). Apache 2.0. Pro features return HTTP 402.

**Pro Edition**:
Built with `-tags pro`. Adds eBPF policy enforcement, SSO/OIDC, telemetry push to VictoriaMetrics, dashboard, and audit logs.

**Feature Gate**:
The `requireFeature(featureName)` Gin middleware that returns 402 Payment Required when a Pro feature is requested on a Community binary.

**MaxNodes**:
The per-license limit on enrolled Peers. Enforced at registration time in `peerService.checkNodeLimit()`.

## Relationships

- A **Workspace** contains zero or more **Peers**
- A **Peer** holds exactly one **Overlay IP** within its Workspace
- An **Enrollment Token** grants access to exactly one **Workspace**
- A **Peer** communicates with other Peers via an **ICE Path** or **LRP Relay**, never both simultaneously
- **Signaling** coordinates **ICE Path** negotiation; it is not on the data path
- **TTFH** is decomposed into: registration latency + signaling round-trips + ICE negotiation + WireGuard handshake
- A **Benchmark Badge** reflects the most recent **Benchmark Suite** run result for one metric

## Example dialogue

> **Dev:** "When a user runs `lattice up`, how long until traffic flows?"
> **Domain expert:** "That depends on the **Connection Scenario**. In a **LAN** scenario the **TTFH SLA** is 3 seconds. In a **NAT** scenario it's 8 seconds because ICE needs to punch through. If ICE fails we fall back to **LRP Relay** and allow up to 15 seconds."
> **Dev:** "Where does that time go exactly?"
> **Domain expert:** "Most of it is **Signaling** — exchanging ICE candidates over NATS. The WireGuard handshake itself is under 500ms once both sides have each other's endpoints."

## Flagged ambiguities

- "node" was used interchangeably with "agent" and "peer" in early code comments — resolved: **Peer** is the canonical term for an enrolled endpoint; "agent" refers only to the `lattice` binary process.
- "connection time" was used loosely — resolved: always use **Time-to-First-Handshake (TTFH)** when referring to the latency SLA metric.
- "network" was ambiguous (could mean Workspace, LatticeNetwork CRD, or underlay) — resolved: **Workspace** for the tenant boundary, **LatticeNetwork** when referring to the CRD specifically.
