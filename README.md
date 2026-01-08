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
  secure,
  low-latency communication.
- [] Intelligent NAT Traversal: Connectivity prioritizes direct P2P tunnels; if direct connection fails, traffic
  seamlessly
  relays via a dedicated TURN/relay server.
- [x] Centralized Orchestration: A Kubernetes-native control plane manages device lifecycle, cryptographic keys, and
  access
  policies, ensuring zero-touch configuration across the entire network.

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
curl -sSL https://raw.githubusercontent.com/wireflowio/wireflow/master/install.sh | bash
```

- specific version: v0.1.0 etc

```bash
curl -sSL https://raw.githubusercontent.com/wireflowio/wireflow/master/install.sh | bash -s -- v0.1.0
```

### Check the installation

using wireflow-cli to check whether both components have installed successfuly:

```bash
wfctl --version
```

## Usage

After the installation, you can use the `wfctl` command to manage your Wireflow network.

### Start wireflow

Just run command as bellow, you will start the wireflow agent on your local machine, if you host name is 'pee1',
peer1 will register to wireflow-controller, you can get the CRDs info using 'kubectl':

```bash
wireflow --log-level=debug
```

on the kubernetes cluster:

```bash
kubectl get wfn
```

Now you can use `wfctl` command to manage your Wireflow network.

### Create a network named 'prod-net'

```bash
wfctl network create prod-net --cidr=10.10.0.0/24
```

### add peer to network

```
wfctl network node add prod-net peer1
```

the peer1 will join the network prod-net successfully, and will get an ip '10.0.0.1' address from the ipam, you can see
it on your kubernetes cluster:

```bash
kubectl wget wfn
```

here, we create a network named 'prod-net' with cidr 10.10.0.0/24, and add peer1 to the network, you can follow the step
to create more peers. all peers will connected to each other automatically.
after you create second peer, you can see the ip address '10.0.0.2' of peer2 on the kubernetes cluster, now you can ping
peer2 from peer1.

```bash
ping 10.0.0.2 -t 10
```

### leave the network
if you want to leave the network, you can run command bellow:
```bash
wfctl network node rm prod-net peer1
```

## Uninstall

After tests, unstall wireflow control-plane components from kubernetes cluster, delete wireflow data-plane directly:
```bash
curl -sSL -f https://raw.githubusercontent.com/wireflowio/wireflow/master/deploy/wireflow.yaml | kubectl delete -f -
`````

For more information, visit [wireflow](https://wireflow.run)

## Building

### Requirements

- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### Steps

**1. Building All**

```bash
git clone https://github.com/wireflowio/wireflow.git
cd wireflow
make build-all
# then install or run the built binaries as needed
```

## Wireflow Features, Roadmap, and Roadmap Progress

**1. Core Features**
These features represent the foundational, working architecture of Wireflow, focusing on security and automation.

- Zero-Touch Onboarding: Users instantly and easily create an encrypted private network without
  requiring any manual tunnel configuration.
- Automatic Enrollment & Autoplay: Devices automatically enroll and configure themselves upon joining, ensuring the
  tunnel is established without manual intervention.
- Security Foundation: Utilizes WireGuard encryption (ChaCha20-Poly1305) with all cryptographic key management
  centralized within the Control Plane.
- Access Control: A robust policy engine is implemented to define granular rules and policies, controlling which devices
  can reach specific endpoints within the network.
- Resilient Connectivity: Implements Relay Fallback to ensure seamless connectivity when direct Peer-to-Peer (P2P)
  connections are blocked by strict NATs or firewalls.

**1. Product Roadmap and Milestones**

- [] Private Service Resolution: Integrated Private DNS service for secure and simplified service/name resolution within
  the overlay network.
- [] Centralized Management: Features a powerful Management API and Web UI with built-in RBAC-ready (Role-Based Access
  Control) access policies.
- [] Operational Visibility: Provides Prometheus-friendly exporters for robust metrics and monitoring integration.
- [x] Flexible Deployment: Easily deployable via Docker; ready-to-use Kubernetes manifests and examples are provided in
  the
  conf/ directory.
- [x] Access control: define rules and policies for who can reach what or where then want
- [] Private DNS: Provides a secure and simplified service discovery mechanism for internal services.

## License

Apache License 2.0



