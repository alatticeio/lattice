# Network Peering Usage Guide

Network peering connects WireGuard networks across different workspaces. Once peered, peers in Workspace A can reach peers in Workspace B at their WireGuard IP addresses — no NAT required, because each workspace's IPAM allocates a unique CIDR from a global pool.

---

## Prerequisites

Before creating a peering, both workspaces must satisfy:

1. **Network is Ready** — the `WireflowNetwork` in each workspace must have `Status.Phase = Ready` and a non-empty `Status.ActiveCIDR`.
2. **A gateway peer exists** in each workspace — one peer must be labeled as the gateway for its network (see below).
3. **IP forwarding is enabled on the gateway node**:
   ```bash
   sysctl -w net.ipv4.ip_forward=1
   ```
   This must be set on the host OS of the peer acting as gateway. Wireflow does not configure this automatically.

---

## Step 1: Designate a Gateway Peer

Each workspace needs exactly one peer acting as the inter-workspace routing hop. The peer must carry two labels:

| Label | Value | Meaning |
|-------|-------|---------|
| `wireflow.run/gateway` | `"true"` | Designates this peer as a gateway |
| `wireflow.run/network-{networkName}` | `"true"` | Associates the gateway with a specific network |

The default network name is `wireflow-default-net`.

### Option A — wireflow CLI

```bash
wireflow peer label <peer-name> \
  -n <namespace> \
  wireflow.run/gateway=true \
  "wireflow.run/network-wireflow-default-net=true"
```

Example (workspace namespace `wf-abc123`):

```bash
wireflow peer label my-gateway-node \
  -n wf-abc123 \
  wireflow.run/gateway=true \
  "wireflow.run/network-wireflow-default-net=true"
```

### Option B — kubectl

```bash
kubectl label wireflowpeer my-gateway-node \
  -n wf-abc123 \
  wireflow.run/gateway=true \
  "wireflow.run/network-wireflow-default-net=true"
```

### Verify

```bash
kubectl get wireflowpeer -n wf-abc123 -l wireflow.run/gateway=true
```

---

## Step 2: Create a Peering

### Via Management UI

1. Open the **Workspace** you want to connect from.
2. Navigate to **Network** → **Peerings** (对等连接).
3. Click **New Peering**.
4. Fill in the form:
   - **Remote Namespace** (`namespaceB`, required): the K8s namespace of the remote workspace (e.g. `wf-xyz789`).
   - **Remote Network** (`networkB`, optional): defaults to `wireflow-default-net`.
   - **Peering Mode** (`peeringMode`, optional): `gateway` (default) or `mesh`.
   - **Name** (optional): auto-generated if left empty.
5. Click **Create**.

### Via API

```bash
curl -X POST https://<management-host>/api/v1/peering \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "namespaceB": "wf-xyz789",
    "networkB": "wireflow-default-net",
    "peeringMode": "gateway"
  }'
```

Response:

```json
{
  "code": 0,
  "data": {
    "name": "peering-abc123-xyz789",
    "local": { "name": "workspace-a", "namespace": "wf-abc123", "cidr": "10.0.1.0/24", "nodeCount": 3 },
    "remote": { "name": "workspace-b", "namespace": "wf-xyz789", "cidr": "10.0.2.0/24", "nodeCount": 2 },
    "status": "pending",
    "peeringMode": "gateway",
    "createdAt": "2026-04-27T10:00:00Z"
  }
}
```

---

## Step 3: Verify the Peering

### Check peering status

```bash
kubectl get wireflownetworkpeering
```

A healthy peering shows `Phase: Ready`:

```
NAME                         STATUS   CIDRA          CIDRB          AGE
peering-abc123-xyz789        Ready    10.0.1.0/24    10.0.2.0/24    2m
```

Status transitions: `Pending → Ready` (or `Error` if a gateway is missing).

### What the controller sets up automatically

Once the `WireflowNetworkPeering` object is created and both gateways are found, the `NetworkPeeringReconciler` performs these steps automatically:

1. **Peering-route annotations on both gateways** — instructs local peers to route the remote CIDR through the gateway peer.
2. **Shadow peers** — creates a synthetic `WireflowPeer` in each namespace representing the remote gateway, with `AllowedIPs` expanded to cover the remote CIDR.
3. **Policies** — creates `WireflowPolicy` objects that admit gateway ↔ shadow peer communication.

After reconciliation, the WireGuard configs on peers are updated automatically on the next `GetNetMap` cycle.

### Verify shadow peers and annotations

```bash
# Shadow peers created by the controller
kubectl get wireflowpeer -n wf-abc123 -l wireflow.run/shadow=true
kubectl get wireflowpeer -n wf-xyz789 -l wireflow.run/shadow=true

# Peering-route annotation on gateway
kubectl get wireflowpeer <gateway-name> -n wf-abc123 -o jsonpath='{.metadata.annotations}'
```

### Test connectivity

From a peer in Workspace A, ping a peer in Workspace B using its WireGuard IP:

```bash
ping 10.0.2.5
```

---

## Peering Modes

| Mode | Behavior | Use case |
|------|----------|----------|
| `gateway` (default) | Traffic is routed through a designated gateway peer in each workspace. Only the two gateways establish a direct WireGuard tunnel. | Production, large workspaces |
| `mesh` | Every peer connects directly to every remote peer. No gateway needed. | Small-scale, low peer count |

---

## List Peerings

### UI

The **Peerings** page lists all peerings for the current workspace, with status, CIDRs, node counts, and creation time.

### API

```bash
curl https://<management-host>/api/v1/peering/list \
  -H "Authorization: Bearer <token>"
```

---

## Delete a Peering

### UI

Click the **Delete** button (trash icon) on the peering row and confirm.

### API

```bash
curl -X DELETE https://<management-host>/api/v1/peering/<peering-name> \
  -H "Authorization: Bearer <token>"
```

### What cleanup does

The controller's finalizer (`wireflow.run/peering-finalizer`) handles cleanup:

1. Removes peering-route annotations from both gateway peers.
2. Deletes shadow peers (`peering-shadow-{name}`) in both namespaces.
3. Deletes peering policies (`wireflow-peering-{name}-*`) in both namespaces.
4. Removes the finalizer and deletes the `WireflowNetworkPeering` object.

Peers on both sides will stop routing to the remote CIDR on their next `GetNetMap` cycle.

---

## Troubleshooting

### Status stays `pending` / `Error`

| Symptom | Cause | Fix |
|---------|-------|-----|
| `Status.Phase = Error` with "no gateway peer found" | No peer labeled `wireflow.run/gateway=true` in one of the namespaces | Label a peer as gateway (Step 1) |
| `Status.Phase = Pending` indefinitely | Network not yet `Ready` (CIDR not allocated) | Check `kubectl get wireflownetwork -n <ns>` |
| Peers can't ping across workspaces | `net.ipv4.ip_forward` not enabled on gateway host | Run `sysctl -w net.ipv4.ip_forward=1` on the gateway node |
| Peering shows `active` but traffic drops | Policy not yet reconciled | Check `kubectl get wireflowpolicy -n <ns>` for `wireflow-peering-{name}-*` objects |

### Check controller logs

```bash
kubectl logs -l app=wireflow-controller -n lattice-system --tail=100 | grep peering
```

---

## Cross-Cluster Peering (Phase 2 — Future)

Cross-cluster peering via `WireflowCluster` + `WireflowClusterPeering` CRDs is planned for a future release. The `GET /api/v1/peering/gateway-info` endpoint is already available for remote clusters to query local gateway information as part of the cross-cluster handshake.
