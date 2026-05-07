# Network Policies

Define label-based access control rules for your mesh network.

## Overview

Lattice enforces **default-deny** networking. You create **allow** policies to permit specific traffic flows between labeled groups of nodes.

## Policy Types

| Type | Description |
|------|-------------|
| Allow | Permit traffic matching specified labels, ports, and protocols |
| Deny | Explicitly block traffic (overrides allows) |

## Policy Structure

A policy specifies:
- **Source labels** — which nodes the traffic comes from
- **Destination labels** — which nodes the traffic goes to
- **Port/Protocol** — which ports and protocols to allow/deny
- **Priority** — higher-priority policies are evaluated first

## Enforcement

| Edition | Backend |
|---------|---------|
| Community | iptables |
| Pro | eBPF (TC ingress on wf0 TUN) |

## Related

- [Nodes & Peers](/features/nodes)
- [Alerts & Rules](/features/alerts)
