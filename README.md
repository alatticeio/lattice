# Wireflow

[![English](https://img.shields.io/badge/lang-English-informational)](README.md) [![中文](https://img.shields.io/badge/语言-中文-informational)](README_zh.md)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Docker Pulls](https://img.shields.io/docker/pulls/wireflow/wireflow.svg?logo=docker&logoColor=white)](https://hub.docker.com/r/wireflow/wireflow)
[![Go Report Card](https://goreportcard.com/badge/github.com/wireflowio/wireflow)](https://goreportcard.com/report/github.com/wireflowio/wireflow)
![Platforms](https://img.shields.io/badge/platforms-windows%20%7C%20linux%20%7C%20macos%20%7C%20android%20%7C%20ios-informational)


## Introduction

Wireflow helps you create a secure private network powered by WireGuard, with a web UI to manage devices, access policies, and connectivity. Connect multiple devices across platforms and centrally control access to your own zero‑config overlay network.
## Technology

- Control plane / Data plane separation
- WireGuard for encrypted tunnels (ChaCha20‑Poly1305)
- Automatic key distribution and rotation via the control plane
- NAT traversal: direct P2P first, relay (TURN) fallback when required
- Peer discovery and connection orchestration engine
- Private DNS for service/name resolution inside the overlay
- Metrics and monitoring (Prometheus‑friendly exporters)
- Management API and Web UI with RBAC‑ready access policies
- Deployable via Docker; Kubernetes manifests and examples in `conf/`
- Cross‑platform agents (Linux, macOS, Windows; mobile in progress)

## Network Topology (High level)

- Devices form a mesh overlay using WireGuard.
- Direct P2P is preferred; if not possible, traffic relays via a TURN/relay node.
- A control plane manages device membership, keys, and policy.


## Quick Start

1. Register an account on Wireflow and sign in.
2. Create a network in the web UI and follow the prompts to add devices.
3. Install and run the Wireflow app/agent on each device, sign in with your account, and join the network.

You should now be able to reach devices over the private network according to the access rules you configured.

## Installation

Choose the method that best fits your environment.

### Docker

```bash
docker run -d \
  --name wireflow \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  -p 51820:51820/udp \
  wireflow/wireflow:latest
```

### Binary (one‑liner)

```bash
bash <(curl -fsSL https://wireflow.io/install.sh)
```

### From source

```bash
git clone https://github.com/wireflowio/wireflow.git
cd wireflow
make build
# then install or run the built binaries as needed
```

### Desktop/Mobile App

Download installers from the website: [wireflow.io](https://wireflow.io)

## Relay (TURN) Overview

If direct P2P connectivity fails (e.g., strict NAT), Wireflow can relay traffic. A free public relay is available for convenience, and you can also deploy your own. You may use the provided relay image or run a compatible TURN server such as `coturn`.

## Deploying a Relay (self‑hosted)

Basic steps:

1. Provision a server with a public IP/UDP open (default 3478/5349 or your chosen port).
2. Deploy the Wireflow relay image or configure `coturn`.
3. In the Wireflow control plane, add your relay endpoint so clients can discover it.

Refer to `conf/` and `turn/` directories in this repo for deployment examples and manifests.

## Features

- [x] Zero‑config onboarding: register, sign in, create a network
- [x] Security: WireGuard encryption and key management
- [x] Access control: define rules and policies for who can reach what
- [x] Web UI: manage devices, rules, and visibility
- [x] Relay fallback: seamless connectivity when direct P2P isn’t possible
- [x] Multi‑platform: Windows, Linux, macOS, Android, iOS, NAS
- [ ] Metrics: traffic, connections, and health insights
- [ ] Multi‑network: manage multiple isolated overlays
- [ ] Docker UI: manage networks without a desktop app
- [ ] DNS: built‑in service and custom domain support

## License

Apache License 2.0



