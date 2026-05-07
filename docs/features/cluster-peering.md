# Cluster Peering

Connect entire Kubernetes clusters across regions or cloud providers.

## Overview

Cluster peering creates a WireGuard bridge between two Kubernetes clusters. All pods in the peered CIDR ranges can communicate as if on the same network.

## How It Works

1. Install the Lattice operator on both clusters
2. Create a `LatticeNetwork` CRD on each cluster
3. The operator establishes a WireGuard tunnel between the two gateways
4. Pod-to-pod traffic routes through the tunnel automatically

## Related

- [Network Peering](/features/network-peering)
- [Relays](/features/relays)
