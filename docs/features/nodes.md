# Nodes & Peers

Manage your WireGuard nodes — view status, join tokens, and peer connectivity.

## Overview

Every device running `lattice` is a **node** (or **peer**). Nodes form the mesh network through WireGuard tunnels.

## Features

- **Node dashboard** — See all nodes, their status (online/offline), IP address, and version
- **Join tokens** — Generate time-bound tokens to enroll new nodes
- **Peer wizard** — Step-by-step onboarding flow for new devices
- **Auto-discovery** — Nodes automatically discover each other via the control plane

## How to Use

1. Go to **Manage → Nodes** to see all nodes
2. Click **Generate Token** to create a join token
3. Run `lattice join --token <token>` on your new device
4. The node appears in the dashboard within seconds

## Related

- [Network Policies](/features/policies)
- [Topology Viewer](/features/topology)
