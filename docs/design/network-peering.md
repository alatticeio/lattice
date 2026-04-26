# Network Peering Design

## Overview

Network peering enables IP-layer connectivity between WireGuard networks that belong to different workspaces (cross-workspace) or different Kubernetes clusters (cross-cluster). Because Wireflow's IPAM assigns each workspace a unique CIDR from a global pool, there are no IP conflicts — peering requires only routing configuration and authorization, not NAT.

---

## Terminology

| Term | Meaning |
|------|---------|
| Workspace | A K8s namespace (`wf-{id}`) containing a WireGuard network and its peers |
| Network | `WireflowNetwork` CRD; each workspace has one default network with an auto-allocated CIDR |
| Gateway Peer | A designated peer within a workspace, labeled `wireflow.run/gateway: "true"`, that acts as the inter-workspace routing hop |
| Shadow Peer | A synthetic `WireflowPeer` created by the peering controller in a remote namespace to represent a foreign gateway |
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
         |                              |
         +----------- WRRP Relay -------+
                    (signaling)
```

### What the peering controller manages

For each `WireflowNetworkPeering{NamespaceA/NetworkA ↔ NamespaceB/NetworkB}`:

1. **Peering-route annotations on gateways** — tells local peers to route the remote CIDR through the gateway.
2. **Shadow peers** — synthetic `WireflowPeer` objects in each namespace representing the remote gateway; these appear in WireGuard config with expanded `AllowedIPs`.
3. **Policies** — `WireflowPolicy` objects that admit shadow peers and gateway peers into `ComputedPeers`.

---

## CRDs

### WireflowNetworkPeering (cluster-scoped)

```yaml
apiVersion: wireflowcontroller.wireflow.run/v1alpha1
kind: WireflowNetworkPeering
metadata:
  name: ws-a-to-ws-b
spec:
  namespaceA: wf-workspace-a
  networkA: wireflow-default-net
  namespaceB: wf-workspace-b
  networkB: wireflow-default-net
  # gateway (default) — traffic transits through a designated gateway peer
  # mesh — every peer connects directly to every remote peer (small scale only)
  peeringMode: gateway
status:
  phase: Ready          # Pending | Ready | Error
  cidrA: 10.0.1.0/24
  cidrB: 10.0.2.0/24
```

### WireflowCluster (cluster-scoped)

Registers a remote Wireflow deployment so `WireflowClusterPeering` can call its management API.

```yaml
apiVersion: wireflowcontroller.wireflow.run/v1alpha1
kind: WireflowCluster
metadata:
  name: cluster-prod-us
spec:
  managementEndpoint: https://wireflow.prod-us.example.com
  credentialRef: cluster-prod-us-token   # Secret in same namespace as controller
status:
  phase: Connected       # Connected | Disconnected | Unknown
  gatewayEndpoint: ""    # populated after first successful handshake
```

### WireflowClusterPeering (cluster-scoped)

```yaml
apiVersion: wireflowcontroller.wireflow.run/v1alpha1
kind: WireflowClusterPeering
metadata:
  name: prod-us-to-prod-eu
spec:
  localNamespace: wf-workspace-a
  localNetwork:   wireflow-default-net
  remoteCluster:  cluster-prod-eu       # ref to WireflowCluster
  remoteNamespace: wf-workspace-x
  remoteNetwork:  wireflow-default-net
status:
  phase: Ready
  localCIDR:  10.0.1.0/24
  remoteCIDR: 10.1.1.0/24
```

---

## Annotations & Labels

| Key | Type | Used on | Meaning |
|-----|------|---------|---------|
| `wireflow.run/gateway` = `"true"` | label | WireflowPeer | Designates this peer as a workspace gateway |
| `wireflow.run/shadow` = `"true"` | label | WireflowPeer | Marks a synthetic shadow peer (skip normal reconcile) |
| `wireflow.run/shadow-allowed-ips` | annotation | shadow WireflowPeer | Extra CIDRs to include in AllowedIPs for this peer (the remote network CIDR) |
| `wireflow.run/peering-route-{peeringName}` | annotation | gateway WireflowPeer | Remote CIDR that local peers should route through this gateway |

---

## Data Plane — Config Generation

### transferToPeer() changes

```
transferToPeer(peer):
  address = peer.Status.AllocatedAddress
  allowedIPs = "{address}/32"

  // Shadow peer: append the remote network CIDR
  if shadowCIDR := peer.Annotations["wireflow.run/shadow-allowed-ips"]; shadowCIDR != "":
    allowedIPs += "," + shadowCIDR

  // Gateway peer: append all peering-route annotations
  for each annotation matching "wireflow.run/peering-route-*":
    allowedIPs += "," + annotationValue
```

### Resulting WireGuard configs

**nodeA1** (normal peer in Workspace A):
```
[Peer] # GatewayA  — has annotation peering-route-{peering}: 10.0.2.0/24
AllowedIPs = 10.0.1.GWA/32, 10.0.2.0/24
```
→ nodeA1 routes all traffic to Workspace B through GatewayA.

**GatewayA** (gateway in Workspace A):
```
[Peer] # shadow-peering-{name}  — shadow of GWB, AllowedIPs annotation = 10.0.2.0/24
AllowedIPs = 10.0.2.GWB/32, 10.0.2.0/24
```
→ GatewayA forwards Workspace B traffic to GatewayB via WireGuard.

**nodeB1** (normal peer in Workspace B):
```
[Peer] # GatewayB  — has annotation peering-route-{peering}: 10.0.1.0/24
AllowedIPs = 10.0.2.GWB/32, 10.0.1.0/24
```

---

## NetworkPeeringReconciler

### Reconcile loop

```
1. Get WireflowNetworkPeering
2. Add finalizer wireflow.run/peering-finalizer if absent
3. If DeletionTimestamp set → cleanup() and remove finalizer
4. Get NetworkA (ns=NamespaceA), check Status.Phase==Ready, Status.ActiveCIDR != ""
5. Get NetworkB (ns=NamespaceB), same check
6. Find GatewayA: WireflowPeer in NamespaceA with labels:
     wireflow.run/gateway=true
     wireflow.run/network-{NetworkA}=true
7. Find GatewayB: same for NamespaceB
8. If either gateway missing → set Status.Phase=Error, requeue after 30s
9. Patch GatewayA annotation wireflow.run/peering-route-{peeringName} = NetworkB.ActiveCIDR
10. Patch GatewayB annotation wireflow.run/peering-route-{peeringName} = NetworkA.ActiveCIDR
11. Create/Update shadow peer of GWA in NamespaceB:
      name: peering-shadow-{peeringName}
      labels: wireflow.run/shadow=true, wireflow.run/network-{NetworkB}=true
      annotations: wireflow.run/shadow-allowed-ips = NetworkA.ActiveCIDR
      spec.PublicKey = GatewayA.Spec.PublicKey
      spec.PeerId   = GatewayA.Spec.PeerId
      spec.AppId    = GatewayA.Spec.AppId
    then Status().Update to set AllocatedAddress = GatewayA.Status.AllocatedAddress
12. Create/Update shadow peer of GWB in NamespaceA (symmetric)
13. Create/Update policies in NamespaceA:
      wireflow-peering-{name}-gw-access: all peers egress to gateway label
      wireflow-peering-{name}-shadow:    gateway peer egress to shadow label
14. Create/Update policies in NamespaceB (symmetric)
15. Set Status.Phase=Ready, CIDRA/CIDRB, Conditions
```

### Cleanup (on deletion)

```
1. Remove annotation wireflow.run/peering-route-{peeringName} from GatewayA and GatewayB
2. Delete shadow peers named peering-shadow-{peeringName} in both namespaces
3. Delete policies named wireflow-peering-{name}-* in both namespaces
4. Remove finalizer
```

### SetupWithManager

- Watches `WireflowNetworkPeering` (cluster-scoped)
- Watches `WireflowPeer` changes → re-enqueue affected peerlings (gateway came online)
- Watches `WireflowNetwork` status changes → re-enqueue when ActiveCIDR becomes available

---

## ClusterPeeringReconciler

### Control plane exchange

```
Cluster A (local)                       Cluster B (remote)
    │                                        │
    │  GET /api/v1/peering/gateway-info      │
    │  ?namespace=wf-ws-x&network=default    │
    │ ──────────────────────────────────────>│
    │                                        │
    │ <──────────────────────────────────────│
    │  { publicKey, gatewayIP, cidr }        │
    │                                        │
    │  POST /api/v1/peering/gateway-info     │
    │  (register self to Cluster B)          │
    │ ──────────────────────────────────────>│
```

### Reconcile loop

```
1. Get WireflowClusterPeering
2. Get WireflowCluster (remoteCluster ref) → managementEndpoint, credentialRef
3. Load credential Secret
4. GET {endpoint}/api/v1/peering/gateway-info?namespace=...&network=...
5. On success:
   a. Create/Update WireflowNetworkPeering in local cluster using:
      - NamespaceA = spec.localNamespace, NetworkA = spec.localNetwork
      - NamespaceB = "cluster-{remoteClusterName}", NetworkB = remoteGatewayInfo.cidr
      - Create synthetic shadow namespace if needed
   b. Create shadow peer representing remote gateway directly in local namespace
6. Update Status
```

### Signaling across clusters

Cross-cluster signaling reuses the existing WRRP relay transport (`PriorityRelay = 50`). Both gateway peers connect to a shared WRRP relay server. No NATS federation is required.

---

## Management API

### GET /api/v1/peering/gateway-info

**Query params:** `namespace`, `network`

**Response:**
```json
{
  "publicKey": "abc123...",
  "gatewayIP":  "10.0.1.5",
  "cidr":       "10.0.1.0/24",
  "appId":      "peer-gateway-xyz"
}
```

Returns the gateway peer info for a network. Used by remote `ClusterPeeringReconciler` to configure cross-cluster peering.

### POST /api/v1/peering (future)

Creates a `WireflowNetworkPeering` between two workspaces in the same cluster.

---

## CIDR Conflict Detection

Since each cluster runs its own IPAM from a separate `WireflowGlobalIPPool`, two clusters could independently allocate overlapping CIDRs (e.g., both use `10.0.0.0/8` with `/24` subnets).

**Detection:** When `ClusterPeeringReconciler` calls the remote gateway-info endpoint, it compares `remoteCIDR` against all local `WireflowSubnetAllocation` records. If any local subnet overlaps with the remote CIDR, the peering is set to `Status.Phase = Error` with condition `CIDRConflict`.

**Resolution:** The admin must reconfigure one cluster's `WireflowGlobalIPPool` to use a non-overlapping base range.

---

## OS-Level Requirements (Gateway Peer)

The gateway peer node must have IP forwarding enabled:

```bash
sysctl -w net.ipv4.ip_forward=1
```

This is a prerequisite for the gateway peer to forward packets between WireGuard interfaces. Wireflow does not configure this automatically — it should be set by the node bootstrap process or flagged by the controller as a readiness requirement.

---

## Implementation Phases

| Phase | Scope | Status |
|-------|-------|--------|
| 1 | `WireflowNetworkPeering` CRD + `NetworkPeeringReconciler` (gateway mode) | This PR |
| 1 | `transferToPeer()` annotation support | This PR |
| 1 | Shadow peer lifecycle management | This PR |
| 1 | Gateway-access policies | This PR |
| 1 | `WireflowCluster` + `WireflowClusterPeering` CRDs | This PR |
| 1 | `ClusterPeeringReconciler` (skeleton + gateway-info HTTP call) | This PR |
| 1 | Gateway-info management API endpoint | This PR |
| 2 | Mesh peering mode | Future |
| 2 | CIDR conflict detection for cross-cluster | Future |
| 2 | Auto gateway selection / HA gateway | Future |
| 2 | Management UI for peering | Future |
