# Wireflow Internal Architecture Guide

## Core Philosophy

### Network as Code
Networking topology is defined via Kubernetes CRDs, eliminating manual tunnel configuration and static firewall rules.

---

### Separation of Planes
* **Control Plane**: Powered by the K8s API; manages identity, IPAM (IP Address Management), and policy orchestration.
* **Data Plane**: Powered by WireGuard + eBPF; handles high-speed encryption and kernel-level access control.

---

### Zero-Trust Networking
No peer is trusted by default. Access is granted based on **identity (Labels)** rather than IP addresses.

## Core Objects
| Object (CRD)               | Scope      | Relationship | Core Responsibility                                                        |
|:---------------------------|:-----------| :--- |:---------------------------------------------------------------------------|
| **WireflowGlobalIPool**    | Cluster    | 1 : N (Namespaces) | Defines the cluster IP Pool, All WireflowNetwork get CIDR from here.       |
| **WireflowNetwork**        | Namespaced | 1 : N (Namespaces) | Defines then namespace Overlay CIDR, Global Routing ID, and MTU settings   |
| **Namespace**              | Namespaced | 1 : 1 (Network) | Logical grouping. Linked to a Network via the label `wireflow.io/network`. |
| **WireflowNetworkPeering** | CLuster    | 1 : N (Namespaces) | Defines two networks connected each other, using NetworkPeering            |
| **WireflowPeer**           | Namespaced | N : 1 (Namespace) | Represents an endpoint (Pod, IoT, PC). Holds PublicKeys and VIPs.          |
| **WireflowPolicy**         | Namespaced | N : 1 (Network) | The "Security Group." Defines Ingress/Egress rules between Peers.          |

##  Data Plane Implementation

## 3.1 Multi-Tenancy via Policy Routing
To run multiple isolated Networks on a single `wg0` interface, Wireflow utilizes **Linux Policy Routing**:

* **FWMARK Tagging**: When a packet enters `wg0` or originates from a Pod, an eBPF program tags the packet with a Network ID (Mark).
* **Isolated Routing Tables**: The kernel selects a specific routing table based on the tag.

```bash
# Each Network has its own dedicated routing table
ip rule add fwmark 100 lookup table 100
```
* **Conflict Resolution** : If two Networks use overlapping CIDRs (e.g., both use 10.0.0.0/24), eBPF performs 1:1 DNAT (Static NAT) at the interface boundary to map them into unique "Shadow IP" spaces during cross-network peering.

## eBPF-Powered Security
WireflowPolicy is compiled into eBPF Maps rather than iptables rules:
- Source Validation: Verifies that the SrcIP of an incoming decrypted packet matches the PublicKey of the sender.
- Line-Speed Filtering: Blocks unauthorized L4 (Port/Protocol) access directly in the kernel, ensuring near-zero latency overhead.
  
## Software Components
### Binary Distribution Strategy
- wireflow-server (The Controller)
    - Dependencies: client-go, controller-runtime.
    - Role: The "Brain." Watches CRDs, runs the Registration API, and manages the global IPAM pool.
- wireflow (The Lean Agent)
    - Dependencies: wireguard-go, ebpf-manager. 
    - Role: The "Muscle." Runs on IoT/PC/Nodes. It communicates with the Server via REST/gRPC (no K8s SDK) to pull configurations and manage the local wg0 interface. 
    - Optimization: By removing K8s dependencies, the binary size is kept under 12MB.

### The "Join" Lifecycle
- Register: User runs wireflow join --network finance --token <T>.
- Auth: The Agent generates a KeyPair, uploads the PublicKey. The Server creates a WireflowPeer resource in the designated Namespace.
- Sync: The Server pushes the Peer list (only for that Network) to the Agent.
- Connect: The Agent configures wg0 and establishes encrypted P2P tunnels.

## Industrial-Grade Connectivity
- NAT Traversal: Integrates STUN/ICE protocols. If peers are behind symmetric NATs, the system automatically fails over to RELAY Mode (via an encrypted TURN-like relay).
- Roaming Support: Leverages WireGuard’s mobility. If a mobile device switches from Wi-Fi to 5G, the tunnel stays alive seamlessly with the same Virtual IP (VIP).

## Developer Maintenance Checklist
- API Evolution: Changes to WireflowNetwork must be backward compatible, as edge agents may run older versions.
- Observability: Use wireflow status for local health checks and kubectl get wfpeer -A for global topology visualization.
- Audit Logs: Denied connections are captured by eBPF and reported asynchronously to the Server for security auditing.