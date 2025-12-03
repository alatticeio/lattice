# Wireflow

[![English](https://img.shields.io/badge/lang-English-informational)](README.md) [![中文](https://img.shields.io/badge/语言-中文-informational)](README_zh.md)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Docker Pulls](https://img.shields.io/docker/pulls/wireflow/wireflow.svg?logo=docker&logoColor=white)](https://hub.docker.com/r/wireflow/wireflow)
[![Go Report Card](https://goreportcard.com/badge/github.com/wireflowio/wireflow)](https://goreportcard.com/report/github.com/wireflowio/wireflow)
![Platforms](https://img.shields.io/badge/platforms-windows%20%7C%20linux%20%7C%20macos%20%7C%20android%20%7C%20ios-informational)

## Introduction

**Wireflow: Kubernetes-Native Network Orchestration using WireGuard.**

Wireflow provides a complete solution for creating and managing a secure, encrypted overlay network powered by
WireGuard.

- Control Plane: The wireflow-controller is the Kubernetes-native component. It continuously watches and reconciles
  Wireflow CRDs (Custom Resource Definitions), serving as the single source of truth for the virtual network state.
- Data Plane: The Wireflow data plane establishes secure, zero-config P2P tunnels across all devices and platforms. It
  receives the desired state from the controller, enabling automated orchestration of connectivity and granular access
  policies.

For more information, please visit our official website: [wireflowio.com](https://wireflowio.com)

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
- Seamless NAT Traversal: Achieves resilient connectivity by prioritizing direct P2P connection attempts, with an
  automated relay (TURN) fallback when required.
- Private Service Resolution: Integrated Private DNS service for secure and simplified service/name resolution within
  the overlay network.

**3.Management & Observability**

- Centralized Management: Features a powerful Management API and Web UI with built-in RBAC-ready (Role-Based Access
  Control) access policies.
- Operational Visibility: Provides Prometheus-friendly exporters for robust metrics and monitoring integration.
- Flexible Deployment: Easily deployable via Docker; ready-to-use Kubernetes manifests and examples are provided in the
  conf/ directory.

Broad Platform Support: Cross-platform agents supporting Linux, macOS, and Windows (with mobile support currently in
progress).

## Network Topology (High-Level Overview)

- P2P Mesh Overlay: Devices automatically form a full mesh overlay network utilizing the WireGuard protocol for secure,
  low-latency communication.
- Intelligent NAT Traversal: Connectivity prioritizes direct P2P tunnels; if direct connection fails, traffic seamlessly
  relays via a dedicated TURN/relay server.
- Centralized Orchestration: A Kubernetes-native control plane manages device lifecycle, cryptographic keys, and access
  policies, ensuring zero-touch configuration across the entire network.

## Quick Start

Follow the steps on: [wireflowio.com](https://wireflowio.com)

## Building / Deploy

## Requirements

- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

## Steps

**1. Building Client**

```bash
git clone https://github.com/wireflowio/wireflow.git
cd wireflow
make build-wireflow
# then install or run the built binaries as needed
```

**2. Building Controller**

```bash
make build-wfctl
# then install or run the built binaries as needed
```

**3. Deploying Wireflow-controller & CRDs**

```bash
make install && make deploy
# 
```

**3. Deploying management / DRP / TURN server**

```bash
make install && make deploy
# 
```

### Uninstall

```bash
make uninstall
```

## Wireflow Components

**1. Wireflow signaling server**

Wireflow signaling server is required for the Wireflow app to work. Which is used to establish peer connections and to
exchange peer metadata.
you can use the public one at https://signaling.wireflow.io, or deploy your own.

**2. Relay (TURN) Overview**

If direct P2P connectivity fails (e.g., strict NAT), Wireflow can relay traffic. A free public relay is available for
convenience, and you can also deploy your own. You may use the provided relay image or run a compatible TURN server such
as `coturn`.

## Deploying a Relay (self‑hosted)

Basic steps:

1. Provision a server with a public IP/UDP open (default 3478/5349 or your chosen port).
2. Deploy the Wireflow relay image or configure `coturn`.
3. In the Wireflow control plane, add your relay endpoint so clients can discover it.

Refer to `conf/` and `turn/` directories in this repo for deployment examples and manifests.

## Wireflow Features, Roadmap, and Roadmap Progress

**1. Core Features**
These features represent the foundational, working architecture of Wireflow, focusing on security and automation.

- Zero-Touch Onboarding: Users can register, sign in, and instantly create an encrypted private network without
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

- [x] Access control: define rules and policies for who can reach what or where then want

## License

Apache License 2.0



