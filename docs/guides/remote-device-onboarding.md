# Remote Device Onboarding

Onboard laptops, edge devices, or home servers into your Lattice mesh.

## Prerequisites

- A running Lattice control plane (see [Quick Start](/guide/quickstart))
- Admin access to generate tokens

## Step 1: Generate a Join Token

In the dashboard: **Manage → Tokens → Generate Token**

Or via CLI:

```bash
lattice token create --network default --ttl 24h
```

Copy the token output (starts with `wf_`).

## Step 2: Install Lattice on the Device

```bash
curl -fsSL https://get.lattice.io | sh
```

## Step 3: Join the Mesh

```bash
lattice join --token wf_xxxx --server https://your-lattice-server:8080
```

You'll see:

```
INF Peer enrolled: 192.168.1.42
INF Tunnel established to control plane
INF Tunnel established to 1 peer
INF Status: ONLINE
```

## Step 4: Verify

Go to **Manage → Nodes** — the new device appears as "online".

## Next Steps

- [Network Policies](/features/policies) — Control what this device can access
- [Topology Viewer](/features/topology) — See your device on the mesh map
