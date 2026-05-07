# Network Peering Design

## Overview

Network peering enables IP-layer connectivity between WireGuard networks that belong to different workspaces (cross-workspace) or different Kubernetes clusters (cross-cluster). Because Lattice's IPAM assigns each workspace a unique CIDR from a global pool, there are no IP conflicts — peering requires only routing configuration and authorization, not NAT.

---

## Terminology

| Term | Meaning |
|------|---------|
| Workspace | A K8s namespace (`wf-{id}`) containing a WireGuard network and its peers |
| Network | `LatticeNetwork` CRD; each workspace has one default network (`lattice-default-net`) with an auto-allocated CIDR |
| Gateway Peer | A designated peer within a workspace, labeled `alattice.io/gateway: "true"`, that acts as the inter-workspace routing hop |
| Shadow Peer | A synthetic `LatticePeer` created by the peering controller in a namespace to represent a remote gateway; skipped by the normal `PeerReconciler` via the `alattice.io/shadow: "true"` label |
| Peering Route | A CIDR annotation added to a gateway peer so other local peers route that CIDR through it |

---

## Problem Statement

**Single-cluster, cross-workspace:**
- Each workspace lives in a separate K8s namespace; a peer in namespace A cannot see peers in namespace B.
- WireGuard AllowedIPs for each peer are only `/32` host routes by default. No routes to a remote CIDR exist.
- There is no authorization mechanism declaring which workspaces may interconnect.

**Cross-cluster:**
- K8s CRDs, NATS signal bus, and IPAM are all cluster-local — there is no shared control plane.
- Peers in Cluster A cannot learn about peers in Cluster B via the existing informer/watch mechanism.

---

## Architecture

### Layers

**Single-cluster, cross-workspace:**

```
+-----------------------+    +-----------------------+
|     Workspace A       |    |     Workspace B       |
|  Network: 10.0.1.0/24 |    |  Network: 10.0.2.0/24 |
|                       |    |                       |
|  nodeA1               |    |  nodeB1               |
|  nodeA2               |    |  nodeB2               |
|  GatewayA  <----------+-WG-+->  GatewayB           |
|  (shadow-GWB)         |    |  (shadow-GWA)         |
+-----------------------+    +-----------------------+
```

**Cross-cluster:**

```
Cluster A                                          Cluster B
+-----------------------------------+    +-----------------------------------+
|  Workspace A (NATS-A)             |    |  Workspace B (NATS-B)             |
|  GatewayA                         |    |  GatewayB                         |
|    │                              |    |      ▲                            |
|    │  ┌──────────────────────┐    |    |      │                            |
|    └─>│  Shared WRRP Relay   │────┼────┼──────┘                            |
|       │  (PriorityRelay=50)  │    |    |                                   |
|       └──────────────────────┘    |    |                                   |
|  (shadow-GWB)                     |    |  (shadow-GWA)                     |
+-----------------------------------+    +-----------------------------------+
         |                                         |
         +── HTTP GET /api/v1/peering/gateway-info ──>
               (control plane: metadata exchange)
```

Two planes operate independently:
- **Control plane**: The local `ClusterPeeringReconciler` makes an HTTP call to the remote management API (`GET /api/v1/peering/gateway-info`) to exchange gateway metadata (public key, CIDR, etc.). No NATS federation is required.
- **Signaling plane**: WireGuard tunnel establishment reuses the shared WRRP relay transport (`PriorityRelay = 50`). Both gateway peers connect to the same WRRP relay server, bridging the separate NATS buses.

### What the peering controller manages

For each `LatticeNetworkPeering{NamespaceA/NetworkA ↔ NamespaceB/NetworkB}`:

1. **Peering-route annotations on gateways** — tells local peers to route the remote CIDR through the gateway.
2. **Shadow peers** — synthetic `LatticePeer` objects in each namespace representing the remote gateway; these appear in WireGuard config with expanded `AllowedIPs`.
3. **Policies** — `LatticePolicy` objects that admit shadow peers and gateway peers into `ComputedPeers`.

---

## CRDs

### LatticeNetworkPeering (cluster-scoped)

```yaml
apiVersion: alattice.io/v1alpha1
kind: LatticeNetworkPeering
metadata:
  name: wf-ws-a-to-wf-ws-b
spec:
  namespaceA: wf-ws-a
  networkA: lattice-default-net
  namespaceB: wf-ws-b
  networkB: lattice-default-net
  # gateway (default) — traffic transits through a designated gateway peer
  # mesh — every peer connects directly to every remote peer (small scale only)
  peeringMode: gateway
status:
  phase: Ready          # Pending | Ready | Error
  cidrA: 10.0.1.0/24
  cidrB: 10.0.2.0/24
```

**ShortName:** `wfpeering`

### LatticeCluster (cluster-scoped)

Registers a remote Lattice deployment so `LatticeClusterPeering` can call its management API.

```yaml
apiVersion: alattice.io/v1alpha1
kind: LatticeCluster
metadata:
  name: cluster-prod-us
spec:
  managementEndpoint: https://lattice.prod-us.example.com
  credentialRef: cluster-prod-us-token   # Secret with label alattice.io/cluster-credential
status:
  phase: Connected       # Connected | Disconnected | Unknown
```

**ShortName:** `wfcluster`

### LatticeClusterPeering (cluster-scoped)

```yaml
apiVersion: alattice.io/v1alpha1
kind: LatticeClusterPeering
metadata:
  name: prod-us-to-prod-eu
spec:
  localNamespace: wf-ws-a
  localNetwork:   lattice-default-net
  remoteCluster:  cluster-prod-eu       # ref to LatticeCluster
  remoteNamespace: wf-ws-x
  remoteNetwork:  lattice-default-net
status:
  phase: Ready
  localCIDR:  10.0.1.0/24
  remoteCIDR: 10.1.1.0/24
```

**ShortName:** `wfcpeering`

---

## Annotations & Labels

| Key | Type | Used on | Meaning |
|-----|------|---------|---------|
| `alattice.io/gateway` = `"true"` | label | LatticePeer | Designates this peer as a workspace gateway |
| `alattice.io/shadow` = `"true"` | label | LatticePeer | Marks a synthetic shadow peer (PeerReconciler skips normal reconciliation) |
| `alattice.io/network-{name}` = `"true"` | label | LatticePeer | Network membership; name is truncated to 63 chars with SHA-256 hash suffix if needed |
| `alattice.io/shadow-allowed-ips` | annotation | shadow LatticePeer | Extra CIDRs to include in AllowedIPs for this peer (the remote network CIDR) |
| `alattice.io/peering-route-{peeringName}` | annotation | gateway LatticePeer | Remote CIDR that local peers should route through this gateway. Name is truncated to 63 chars with SHA-256 hash suffix if needed |

### Finalizers

| Finalizer | Used on | Purpose |
|-----------|---------|---------|
| `alattice.io/peering-finalizer` | LatticeNetworkPeering | Ensures shadow peers and policies are cleaned up on deletion |
| `alattice.io/cluster-peering-finalizer` | LatticeClusterPeering | Ensures shadow peers and policies are cleaned up on deletion |

---

## Data Plane — Config Generation

### Peer State Machine

The `PeerReconciler` manages each `LatticePeer` through a phase-based state machine:

```
(empty) --initialize--> Pending --join network--> Ready <--spec change-- Failed
                            ^                                                         |
                            +------------------- reset (30s backoff) -----------------+
```

- **Initialization**: Generate WireGuard keys (private/public/PeerId), advance to `Pending`.
- **Pending**: Join network (allocates IP via IPAM), advance to `Ready`.
- **Ready**: Call `lastReconcile()` to build WireGuard config and update ConfigMap. Detect spec changes (network transition).
- **Failed**: Reset to `Pending` with 30s backoff.

Shadow peers (`alattice.io/shadow: "true"`) are skipped immediately at the top of the reconcile loop — they are managed exclusively by `NetworkPeeringReconciler` / `ClusterPeeringReconciler`.

### ConfigMap Generation

The `lastReconcile()` function:

1. Builds `PeerStateSnapshot` (peer, network, all peers in network, matching policies).
2. Calls `generator.generate()` which:
   - Transfers current peer via `transferToPeer()`.
   - Collects all peers in the network (skips those without `AllocatedAddress`).
   - Calls `peerResolver.ResolvePeers()` to compute `ComputedPeers` from policies.
   - Calls `firewallResolver.ResolveRules()` to compute `ComputedRules`.
3. Creates/updates ConfigMap named `<peer.Name>-config` with `config.json` containing the serialized `infra.Message`.
4. Hash comparison: `alattice.io/config-hash` annotation vs `Status.CurrentHash` — skips update if unchanged.

### transferToPeer()

```
transferToPeer(peer):
  address = peer.Status.AllocatedAddress
  allowedIPs = "{address}/32"

  // Shadow peer: append the remote network CIDR
  if shadowCIDR := peer.Annotations["alattice.io/shadow-allowed-ips"]; shadowCIDR != "":
    allowedIPs += "," + shadowCIDR

  // Gateway peer: append all peering-route annotations
  for each annotation matching "alattice.io/peering-route-*":
    allowedIPs += "," + annotationValue
```

### Resulting WireGuard configs

**nodeA1** (normal peer in Workspace A):
```
[Peer] # GatewayA  — has annotation alattice.io/peering-route-{peeringName}: 10.0.2.0/24
AllowedIPs = 10.0.1.GWA/32, 10.0.2.0/24
```
→ nodeA1 routes all traffic to Workspace B through GatewayA.

**GatewayA** (gateway in Workspace A):
```
[Peer] # peering-shadow-{name}  — shadow of GWB, AllowedIPs annotation = 10.0.2.0/24
AllowedIPs = 10.0.2.GWB/32, 10.0.2.0/24
```
→ GatewayA forwards Workspace B traffic to GatewayB via WireGuard.

**nodeB1** (normal peer in Workspace B):
```
[Peer] # GatewayB  — has annotation alattice.io/peering-route-{peeringName}: 10.0.1.0/24
AllowedIPs = 10.0.2.GWB/32, 10.0.1.0/24
```

---

## NetworkPeeringReconciler

### Reconcile loop

```
1. Get LatticeNetworkPeering
2. Add finalizer alattice.io/peering-finalizer if absent
3. If DeletionTimestamp set → cleanup() and remove finalizer
4. Get NetworkA (ns=NamespaceA), check Status.Phase==Ready, Status.ActiveCIDR != ""
5. Get NetworkB (ns=NamespaceB), same check
6. Find GatewayA: LatticePeer in NamespaceA with labels:
     alattice.io/gateway=true
     alattice.io/network-{NetworkA}=true
7. Find GatewayB: same for NamespaceB
8. If either gateway missing → set Status.Phase=Error, requeue after 30s
9. Patch GatewayA annotation alattice.io/peering-route-{peeringName} = NetworkB.ActiveCIDR
10. Patch GatewayB annotation alattice.io/peering-route-{peeringName} = NetworkA.ActiveCIDR
11. Create/Update shadow peer of GWA in NamespaceB:
      name: peering-shadow-{peeringName}
      labels: alattice.io/shadow=true, alattice.io/network-{NetworkB}=true
      annotations: alattice.io/shadow-allowed-ips = NetworkA.ActiveCIDR
      spec.PublicKey = GatewayA.Spec.PublicKey
      spec.PeerId   = GatewayA.Spec.PeerId
      spec.AppId    = GatewayA.Spec.AppId
    then Status().Update to set AllocatedAddress = GatewayA.Status.AllocatedAddress
12. Create/Update shadow peer of GWB in NamespaceA (symmetric)
13. Create/Update policies in NamespaceA:
      lattice-peering-{name}-gw-access:
        PeerSelector: {} (all peers)
        Egress: alattice.io/gateway=true
        Ingress: alattice.io/gateway=true
      lattice-peering-{name}-shadow:
        PeerSelector: alattice.io/gateway=true
        Egress: alattice.io/shadow=true
14. Create/Update policies in NamespaceB (symmetric)
15. Set Status.Phase=Ready, CIDRA/CIDRB, Conditions
```

### Cleanup (on deletion)

```
1. Remove annotation alattice.io/peering-route-{peeringName} from GatewayA and GatewayB
2. Delete shadow peers named peering-shadow-{peeringName} in both namespaces
3. Delete policies named lattice-peering-{name}-* in both namespaces
4. Remove finalizer
```

### SetupWithManager

- Watches `LatticeNetworkPeering` (generation change)
- Watches `LatticePeer` changes → re-enqueue affected peerings (gateway came online)
- Watches `LatticeNetwork` status changes → re-enqueue when ActiveCIDR becomes available

---

## ClusterPeeringReconciler

### Control plane exchange

Cluster A's `ClusterPeeringReconciler` makes a one-way HTTP call to Cluster B's management API to fetch gateway info. The flow is **uni-directional** from each cluster's perspective — bidirectional peering requires creating a symmetric `LatticeClusterPeering` on Cluster B pointing back to Cluster A.

```
Cluster A (local)                       Cluster B (remote)
    │                                        │
    │  GET /api/v1/peering/gateway-info      │
    │  ?namespace=<remoteNs>&network=<net>   │
    │  Authorization: Bearer <token>         │
    │ ──────────────────────────────────────>│
    │                                        │
    │ <──────────────────────────────────────│
    │  { publicKey, gatewayIP, cidr, appId, peerId }
    │                                        │
```

Credential loading:
- The bearer token is loaded from a `Secret` with label `alattice.io/cluster-credential=<secretName>`, key `token`.
- The `LatticeCluster` CR holds `ManagementEndpoint` (HTTPS URL) and `CredentialRef` (Secret name).

### Reconcile loop

```
1. Get LatticeClusterPeering
2. Get LatticeCluster (spec.remoteCluster ref) → managementEndpoint, credentialRef
3. Load bearer token from Secret with label alattice.io/cluster-credential=<secretName>, key "token"
4. GET {managementEndpoint}/api/v1/peering/gateway-info?namespace={remoteNamespace}&network={remoteNetwork}
   with Authorization: Bearer {token}
5. Verify local network is Ready with ActiveCIDR
6. Create/Update shadow peer in local namespace:
      name: cluster-shadow-{cp.Name}
      labels: alattice.io/shadow=true, alattice.io/network-{localNetwork}=true
      annotations: alattice.io/shadow-allowed-ips = remoteCIDR
      spec.PublicKey = info.PublicKey
      spec.PeerId   = info.PeerID
      spec.AppId    = info.AppID
    then Status().Update to set AllocatedAddress = info.GatewayIP
7. Find local gateway (alattice.io/gateway=true, alattice.io/network-{localNetwork}=true)
8. Patch local gateway annotation alattice.io/peering-route-{cp.Name} = remoteCIDR
9. Create/Update policies in local namespace:
      lattice-cpeering-{name}-gw-access:
        PeerSelector: {} (all peers)
        Egress: alattice.io/gateway=true
        Ingress: alattice.io/gateway=true
      lattice-cpeering-{name}-shadow:
        PeerSelector: alattice.io/gateway=true
        Egress: alattice.io/shadow=true
10. Update Status.Phase=Ready, LocalCIDR, RemoteCIDR, Conditions
```

### Cleanup (on deletion)

```
1. Remove annotation alattice.io/peering-route-{cp.Name} from local gateway
2. Delete shadow peer named cluster-shadow-{cp.Name} in local namespace
3. Delete policies named lattice-cpeering-{name}-* in local namespace
4. Remove finalizer
```

**Key difference from within-cluster peering:** The shadow peer is created **only on the local side**. The remote side independently creates its own shadow peer for the local gateway via its own `LatticeClusterPeering` resource. This is a uni-directional control from each cluster's perspective.

### WRRP relay for cross-cluster signaling

Each cluster runs its own NATS message bus, so peers in different clusters cannot exchange WireGuard signaling directly. Both gateway peers connect to a shared WRRP relay server (configured via `LatticeRelayServer`, priority `PriorityRelay = 50`). The relay bridges the separate NATS buses, enabling WireGuard tunnel establishment between the two gateways. No NATS federation or additional signaling infrastructure is required.

---

## Management API

### GET /api/v1/peering/gateway-info

**Auth:** No authentication required (used by remote clusters for cross-cluster peering; security is via the bearer token in the credential Secret).

**Query params:** `namespace` (required), `network` (required)

**Response:**
```json
{
  "publicKey": "abc123...",
  "gatewayIP":  "10.0.1.5",
  "cidr":       "10.0.1.0/24",
  "appId":      "lattice-peer-gateway-xyz",
  "peerId":     "peer-abc123"
}
```

Returns the gateway peer info for a network. Validates that the network is `NetworkPhaseReady` with non-empty `ActiveCIDR`. Finds gateway peer via labels `alattice.io/gateway=true` + `alattice.io/network-{networkName}=true`. Errors if no gateway peer exists or gateway has no `AllocatedAddress` yet.

### Peering CRUD endpoints (under /api/v1/peering)

| Method | Path | Middleware | Description |
|--------|------|------------|-------------|
| GET | `/api/v1/peering/list` | auth + tenant | List peerings for current workspace |
| POST | `/api/v1/peering` | auth + tenant | Create a peering |
| DELETE | `/api/v1/peering/:name` | auth + tenant | Delete a peering |

**Create service logic:**
- `NamespaceA` = current workspace's namespace (from tenant context).
- `NamespaceB` = from request body.
- `NetworkA` = defaults to first network in namespace, preferring `lattice-default-net`.
- `NetworkB` = defaults to `lattice-default-net`.
- Name auto-generated as `<nsA>-to-<nsB>` (lowercased, underscores replaced with hyphens) if not provided.
- `PeeringMode` defaults to `gateway` if empty.
- Creates `LatticeNetworkPeering` CR directly.

---

## CIDR Conflict Detection

Since each cluster runs its own IPAM from a separate `LatticeGlobalIPPool`, two clusters could independently allocate overlapping CIDRs (e.g., both use `10.0.0.0/8` with `/24` subnets).

**Detection:** When `ClusterPeeringReconciler` calls the remote gateway-info endpoint, it compares `remoteCIDR` against all local `LatticeSubnetAllocation` records. If any local subnet overlaps with the remote CIDR, the peering is set to `Status.Phase = Error` with condition `CIDRConflict`.

**Resolution:** The admin must reconfigure one cluster's `LatticeGlobalIPPool` to use a non-overlapping base range.

---

## OS-Level Requirements (Gateway Peer)

The gateway peer node must have IP forwarding enabled:

```bash
sysctl -w net.ipv4.ip_forward=1
```

This is a prerequisite for the gateway peer to forward packets between WireGuard interfaces. Lattice does not configure this automatically — it should be set by the node bootstrap process or flagged by the controller as a readiness requirement.

---

## Implementation Phases

| Phase | Scope | Status |
|-------|-------|--------|
| 1 | `LatticeNetworkPeering` CRD + `NetworkPeeringReconciler` (gateway mode) | Implemented |
| 1 | `transferToPeer()` annotation support | Implemented |
| 1 | Shadow peer lifecycle management | Implemented |
| 1 | Gateway-access policies | Implemented |
| 1 | `LatticeCluster` + `LatticeClusterPeering` CRDs | Implemented |
| 1 | `ClusterPeeringReconciler` (gateway-info HTTP call) | Implemented |
| 1 | Gateway-info management API endpoint | Implemented |
| 2 | Mesh peering mode (CRD field exists but reconcile logic not implemented) | Future |
| 2 | CIDR conflict detection for cross-cluster | Future |
| 2 | Auto gateway selection / HA gateway | Future |
| 2 | Management UI for peering | Future |
