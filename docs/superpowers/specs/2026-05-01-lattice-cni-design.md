# Lattice CNI Plugin Design

**Date**: 2026-05-01
**Status**: Draft — pending implementation plan

## 1. Overview

Lattice CNI is a chained CNI plugin that connects Kubernetes Pods to the Lattice overlay network. It runs alongside an existing primary CNI (Calico/Flannel/Cilium) and is invoked by Multus as a secondary network.

**Goals:**
- Pods get an overlay VIP from Lattice's IPAM pool (e.g., `10.10.x.x`)
- Overlay traffic route through the host's `wf0` TUN interface → WireGuard → remote clusters
- Cross-cluster CIDR conflicts are resolved by overlay VIP mapping (DNS-based service discovery, phase 2)
- Reuse existing eBPF/iptables policy engine for Pod-to-overlay access control

**Non-goals:**
- Replacing the primary CNI (phase 1; planned for later)
- Managing cross-cluster service discovery (phase 2)

## 2. Architecture

```
┌──────────────────────────────────────────────────────────┐
│  K8s Node                                                 │
│                                                          │
│  ┌────────────────┐     ┌────────────────┐               │
│  │  Pod A         │     │  Pod B          │               │
│  │  eth0 (primary CNI) │  eth0 (primary CNI)             │
│  │  lth0 (CNI)    │     │  lth0 (CNI)     │               │
│  └──┬─────────────┘     └──┬──────────────┘               │
│     │ veth lthX            │ veth lthX                    │
│     ▼                      ▼                              │
│  ┌──────────────────────────────────────────────────┐    │
│  │  Host netns                                       │    │
│  │  wf0 (TUN, Lattice Agent)                        │    │
│  │  ┌─────────────────────────────────────────┐     │    │
│  │  │ WireGuard + ICE(P2P) + WRRP(Relay)       │     │    │
│  │  │ eBPF/iptables policy engine              │     │    │
│  │  └─────────────────────────────────────────┘     │    │
│  │  IPAM Daemon (localhost socket)                   │    │
│  └──────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────┘
```

**Traffic path (outbound):**
```
Pod(lth0) → veth → host(lthX) → kernel route match → wf0(TUN) → WireGuard → ICE/WRRP → remote
```

**Traffic path (inbound):**
```
remote → WireGuard → wf0(TUN) → kernel route → host(lthX) → veth → Pod(lth0)
```

## 3. Components

### 3.1 Lattice CNI Plugin (`internal/agent/cni/`)

Standard CNI plugin binary implementing `ADD`, `DEL`, `CHECK` interfaces.

**Responsibilities:**
- Create veth pair (`lthX` on host, `lth0` in container)
- Call IPAM Daemon to allocate/release VIP
- Configure Pod netns (IP, routes on `lth0`)
- Configure host veth endpoint
- Call Agent to create/remove policy rules for the Pod

**CNI config** (`/etc/cni/net.d/10-lattice.conflist`):
```json
{
  "cniVersion": "0.4.0",
  "name": "lattice-overlay",
  "type": "lattice-cni",
  "ipam": {
    "type": "lattice-ipam",
    "daemonSocket": "/run/lattice/ipam.sock"
  },
  "agentSocket": "/run/lattice/agent.sock",
  "overlayCIDR": "10.10.0.0/8",
  "mtu": 1420
}
```

### 3.2 IPAM Daemon (`internal/agent/cniipam/`)

gRPC service running inside the Lattice DaemonSet Pod.

**Responsibilities:**
- Receive IPAM requests from CNI plugin via localhost socket
- Allocate/release IPs from the LatticeNetwork's /24 pool
- Track endpoint-to-Pod mappings

### 3.3 Lattice Agent (existing, extended)

The host-level lattice agent already manages `wf0`, WireGuard, ICE, WRRP, and policy enforcement. For CNI:

- Exposes a local socket for IPAM allocation requests
- Exposes a local socket for Pod policy rule creation/removal
- Extends the existing `Provisioner` interface to handle veth endpoints

## 4. CNI Lifecycle

### ADD Flow

1. Multus invokes `lattice-cni ADD` with `(ContainerID, NetNS, IfName, CNI args)`
2. CNI creates veth pair: `lthX` (host) / `lth0` (container)
3. CNI moves `lth0` into Pod netns
4. CNI calls IPAM Daemon → allocates VIP (`10.10.1.50/24`) + host-veth IP (`10.10.1.49/30`)
5. CNI configures Pod netns:
   - `ip addr add 10.10.1.50/24 dev lth0`
   - `ip route add <overlay-cidr> dev lth0 via 10.10.1.49`
6. CNI configures host:
   - `ip addr add 10.10.1.49/30 dev lthX`
   - `ip link set lthX up`
   - Enable `net.ipv4.ip_forward` (if not already)
7. CNI calls Agent to create policy rules (eBPF/iptables on `wf0` or `lthX`)
8. CNI returns result to Multus

### DEL Flow

1. Multus invokes `lattice-cni DEL`
2. CNI calls IPAM Daemon → release VIP
3. CNI calls Agent → remove policy rules
4. CNI deletes veth pair

## 5. Cross-Cluster CIDR Conflict Resolution

Two clusters with overlapping Pod CIDRs (e.g., both `10.42.0.0/16`) need a way to route to each other.

**Approach**: Each Pod gets an overlay VIP that is globally unique within the Lattice network. Cross-cluster communication uses overlay VIPs, not original Pod IPs.

**Phase 1** (this design): Overlay VIPs are allocated and routable. Manual configuration of remote endpoints.

**Phase 2** (future): DNS-based service discovery.
- Lattice DNS server resolves `svc.namespace.cluster-lattice.local` → overlay VIP
- Same `svc.namespace.svc.cluster.local` resolves to local Pod IP (primary CNI)
- Applications choose the appropriate DNS name based on whether they want local or cross-cluster access

## 6. Error Handling

| Scenario | Behavior |
|----------|----------|
| IPAM Daemon unreachable | CNI returns error; Pod scheduling falls back to primary CNI (via Multus `defaultNetwork`) |
| `wf0` not ready on node | Reject ADD, log event to Pod status |
| veth name conflict (netns already has same interface) | Add random suffix, retry up to 3 times |
| CNI DEL lost (Pod deleted without cleanup) | IPAM retains IP lease with 24h TTL; next same-Pod ADD reuses it |
| Policy engine fails to load | Reject Pod from overlay (fail-secure); no fallback to unfiltered access |

## 7. Testing

| Type | Scope |
|------|-------|
| Unit | IPAM allocation logic, CNI config parsing, veth name conflict retry |
| Integration | kind cluster + Multus + CNI ADD/DEL, verify Pod netns routing correctness |
| E2E | Two kind clusters, cross-cluster ping overlay VIP, verify wf0 traffic + policy enforcement |

## 8. New Code Structure

```
internal/agent/cni/
├── plugin/
│   ├── main.go              # CNI binary entry point (skel.PluginMain)
│   ├── cmd_add.go           # ADD implementation
│   ├── cmd_del.go           # DEL implementation
│   └── config.go            # NetConf struct
├── cniipam/
│   ├── daemon.go            # IPAM gRPC server, listens on localhost socket
│   ├── allocator.go         # Calls Lattice IPAM for allocate/release
│   └── types.go             # IPAM request/response types
└── provision/
    └── veth_policy.go       # Extends Provisioner for Pod veth policy rules

Makefile targets:
- make build-cni       # Build CNI plugin binary → dist/lattice-cni
- make install-cni     # Copy binary to /opt/cni/bin/ (dev)
- EDITION=pro make build-cni  # PRO build with eBPF policy support
```

## 9. Deployment

Lattice CNI is deployed as a DaemonSet:
- Container: same image as lattice agent
- Volume mounts:
  - `/opt/cni/bin` (for lattice-cni binary)
  - `/etc/cni/net.d` (for 10-lattice.conflist)
  - `/run/lattice` (for IPAM/agent sockets)
  - Pod netns access (via hostPath `/var/run/netns`)

Multus configuration (NetworkAttachmentDefinition):
```yaml
apiVersion: k8s.cni.cni.dev/v1
kind: NetworkAttachmentDefinition
metadata:
  name: lattice-overlay
spec:
  config: |
    {
      "cniVersion": "0.4.0",
      "name": "lattice-overlay",
      "type": "lattice-cni",
      "ipam": {
        "type": "lattice-ipam",
        "daemonSocket": "/run/lattice/ipam.sock"
      },
      "agentSocket": "/run/lattice/agent.sock",
      "overlayCIDR": "10.10.0.0/8",
      "mtu": 1420
    }
```
