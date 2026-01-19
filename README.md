## Wireflow - Cloud Native WireGuard Management Platform

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/wireflowio/wireflow)](https://goreportcard.com/report/github.com/wireflowio/wireflow)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

> ⚠️ **Early Alpha**: This project is under active development.
> APIs may change. Not production-ready yet.

## Introduction

**Wireflow: Kubernetes-Native Network Orchestration using WireGuard.**

Wireflow provides a complete solution for creating and managing a secure, encrypted overlay network powered by
WireGuard.

- Control Plane: The wireflow-controller is the Kubernetes-native component. It continuously watches and reconciles
  Wireflow CRDs (Custom Resource Definitions), serving as the single source of truth for the virtual network state.
- Data Plane: The Wireflow data plane establishes secure, zero-config P2P tunnels across all devices and platforms. It
  receives the desired state from the controller, enabling automated orchestration of connectivity and granular access
  policies.

For more information, please visit our official website: [wireflow.run](https://wireflow.run)

## Wireflow Technical Capabilities

**1. Architecture & Core Security**

- Decoupled Architecture: Clear Control Plane / Data Plane separation for enhanced scalability, performance, and
  security.
- High-Performance Tunnels: Utilizes WireGuard for secure, high-speed encrypted tunnels (ChaCha20-Poly1305).
- Zero-Touch Key Management: Automatic key distribution and rotation, with zero-touch provisioning handled entirely by
  the Control Plane.

**2.Kubernetes & Networking Automation**

- Kubernetes-Native Orchestration: Peer discovery and connection orchestration are managed directly through a
  Kubernetes-native CRDs controller.
- Seamless NAT Traversal: Achieves resilient connectivity by prioritizing direct P2P connection attempts, in future with
  an
  automated relay (TURN) fallback when required.

Broad Platform Support: Cross-platform agents supporting Linux, macOS, and Windows (with mobile support currently in
progress).

## Network Topology (High-Level Overview)

- [x] P2P Mesh Overlay: Devices automatically form a full mesh overlay network utilizing the WireGuard protocol for
  secure, low-latency communication.
- [] Intelligent NAT Traversal: Connectivity prioritizes direct P2P tunnels; if direct connection fails, traffic
  seamlessly relays via a dedicated TURN/relay server.
- [x] Centralized Orchestration: A Kubernetes-native control plane manages device lifecycle, cryptographic keys, and
  access policies, ensuring zero-touch configuration across the entire network.

**Key Features:**

- [x] Kubernetes CRD-based configuration
- [x] Automatic IP allocation (IPAM)
- [] Multi-cloud/hybrid-cloud support
- [x] Built on WireGuard (fast & secure)
- [] GitOps ready

## Quick Start

### Install control-plane

you should have a kubernetes cluster with kubectl configured:

```bash
curl -sSL https://raw.githubusercontent.com/wireflowio/wireflow/master/deploy/wireflow.yaml | kubectl apply -f - 
```

### Install data-plane

- latest version

```bash
curl -sSL https://raw.githubusercontent.com/wireflowio/wireflow/master/hack/install.sh | bash
```

- specific version: v0.1.0

```bash
curl -sSL https://raw.githubusercontent.com/wireflowio/wireflow/master/hack/install.sh | bash -s -- v0.1.0
```

### Check the installation
> - Note: After installation, you can use the wireflow command to check the version. Before doing so, you must set the signaling server address. The default is nats://signaling.wireflow.run:4222.
> - If you are using a custom NATS server (e.g., your Kubernetes node IP), use the following command:
> ```bash
> wireflow --signaling-url=nats://your-nats-ip:4222 --version
> ```
> To make this change permanent so you don't have to type it every time, use the config command:
> ```bash
> wireflow config set signaling-url nats://your-nats-ip:4222
> ```

Now you can use `wireflow` to check whether both components have installed successfuly:

```bash
wireflow --version
```

### Start The Wireflow Agent

Run the following command to start the Wireflow agent on your local machine.

```bash
wireflow up --level=debug --token=YOUR_TOKEN
```

### Token Management:

- **Automatic Generation**: If no token is provided during the first connection, the Control Plane will automatically
  generate one for your peer.
- **Persistence**: The token will be returned to your peer and saved automatically to ~/.wireflow.yaml.
- **Auto-load**: On subsequent restarts, the agent will automatically load the token from the configuration file if the
  --token flag is omitted.
- **Manual Override**: You can also manually specify the token using the --token flag.

when another peer want to join the network that first peer created, just run below command:
> Note: PEER_TOKEN is the token of first peer

```bash
wireflow up --level=debug --token=PEER_TOKEN
```

### Check the network status
1. View WireGuard Statistics
wireflow integrates seamlessly with the standard WireGuard toolset. Use the wg command to inspect connection details, such as public keys, endpoints, and data transfer volumes:

```bash
# Display the status of all wireflow-managed interfaces
wg
```

2. Test Peer Connectivity
To ensure the encrypted tunnel is passing traffic correctly, use ping to reach a remote peer by its internal IP defined in your wireflow network:

```bash
# Ping a remote peer (replace 'peer1' with your peer's name or IP)
ping -c 3 peer1
```

### Uninstall

To remove the Control Plane and cleanup:

```bash
curl -sSL -f https://raw.githubusercontent.com/wireflowio/wireflow/master/deploy/wireflow.yaml | kubectl delete -f -
`````

For more information, visit [wireflow](https://wireflow.run)

## Development

### Requirements

- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### Build from source:

```bash
git clone https://github.com/wireflowio/wireflow.git
cd wireflow
make build-all
```

## Badges

![Build Status](https://github.com/wireflowio/wireflow/workflows/CI/badge.svg)
[![License](https://img.shields.io/github/license/wireflowio/wireflow)](/LICENSE)
[![Release](https://img.shields.io/github/release/wireflowio/wireflow.svg)](https://github.com/golangci/golangci-lint/releases/latest)
[![Docker](https://img.shields.io/docker/pulls/wireflowio/wireflow)](https://hub.docker.com/r/wireflowio/wireflow)
[![GitHub Releases Stats of wireflow](https://img.shields.io/github/downloads/wireflowio/wireflow/total.svg?logo=github)](https://somsubhra.github.io/github-release-stats/?username=wireflowio&repository=wireflow)

## Contributors

This project exists thanks to all the people who contribute. [How to contribute](https://wireflow.run).

<a href="https://github.com/wireflowio/wireflow/graphs/contributors">
  <img src="https://opencollective.com/wireflowio/contributors.svg?width=890&button=false&skip=golangcidev,CLAassistant,renovate,fossabot,golangcibot,kortschak,golangci-releaser,dependabot%5Bbot%5D" />
</a>

## Wireflow Features, Roadmap, and Roadmap Progress

**1. Core Features (Available)**
These features represent the foundational, working architecture of Wireflow, focusing on security and automation.

- [x] Zero-Touch Onboarding: Users instantly and easily create an encrypted private network without
  requiring any manual tunnel configuration.
- [x] Automatic Enrollment & Autoplay: Devices automatically enroll and configure themselves upon joining, ensuring the
  tunnel is established without manual intervention.
- [x] Security Foundation: Utilizes WireGuard encryption (ChaCha20-Poly1305) with all cryptographic key management
  centralized within the Control Plane.
- [x] Kubernetes-Native Orchestration: Peer discovery and connection orchestration are managed directly through a
  Kubernetes-native CRDs controller.
- [x] Native Kubernetes Support: Wireflow is designed to run natively within Kubernetes, eliminating the need for
  additional orchestration layers.
- [x] Native Networking Support: Wireflow leverages Kubernetes networking primitives to provide a seamless,
  transparent overlay network.
- [x] Native Access Control: Wireflow provides a simple, declarative access policy model for controlling peer access.
- [x] Native Device Discovery: Wireflow leverages Kubernetes node labels to automatically discover and connect to
  devices.
- [x] Native Device Management: Wireflow provides a simple, declarative device management model for managing peer
  lifecycle.
- [x] IPAM Support: Wireflow Create an IPAM to automatically allocate network for tenant and allocate IP addresses for each peer.


**2. Future Milestones (Planned)**

- [] Multi-Cloud Support: Wireflow supports hybrid cloud deployments, allowing users to connect to their devices from
  multiple cloud providers.
- [] Multi-Region Support: Wireflow supports multi-region deployments, allowing users to connect to their devices from
  multiple regions.
- [] Multi-Tenant Support: Wireflow supports multi-tenant deployments, allowing users to connect to their devices from
  multiple tenants.
- [] Centralized Management: Features a powerful Management API and Web UI with built-in RBAC-ready (Role-Based Access
  Control) access policies.
- [] Operational Visibility: Provides Prometheus-friendly exporters for robust metrics and monitoring integration.
- [] Native DNS: Provides a secure and simplified service discovery mechanism for internal services.

## License

Apache License 2.0



