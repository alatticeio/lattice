<div align="center">

# Wireflow

**Cloud-Native WireGuard Network Orchestration**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/wireflowio/wireflow)](https://goreportcard.com/report/github.com/wireflowio/wireflow)
[![Release](https://img.shields.io/github/v/release/wireflowio/wireflow)](https://github.com/wireflowio/wireflow/releases/latest)
[![Docker](https://img.shields.io/docker/pulls/wireflowio/wireflow)](https://hub.docker.com/r/wireflowio/wireflow)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

Wireflow simplifies the construction of encrypted overlay networks across multi-cloud, cross-datacenter, and edge environments — without touching firewalls or exposing public IPs.

[**Website**](https://wireflow.run) · [**Documentation**](https://wireflow.run/docs) · [**Issues**](https://github.com/wireflowio/wireflow/issues)

</div>

---

## Overview

Wireflow is a WireGuard management platform built for Kubernetes. It automates the full lifecycle of secure peer-to-peer tunnels:

- **Control Plane** — Kubernetes Operator that declaratively manages network topology via CRDs. Acts as the single source of truth for keys, IP allocation, and peer relationships.
- **Data Plane** — Lightweight agent deployed on each device. Establishes encrypted WireGuard tunnels with automatic NAT traversal (ICE/STUN/TURN), even across symmetric NATs.
- **Relay Plane** — Built-in WRRP relay server as fallback when direct P2P is not possible.

## Architecture

![Architecture](docs/images/architecture.png)

## Features

| Feature | Status |
|---------|--------|
| WireGuard tunnel automation (key distribution, rotation) | ✅ |
| Automatic NAT traversal (ICE / STUN / TURN) | ✅ |
| Built-in IPAM — conflict-free IP allocation per workspace | ✅ |
| CRD-based declarative network topology | ✅ |
| Network policy engine (allow/deny, ingress/egress, port-level) | ✅ |
| Multi-workspace & RBAC | ✅ |
| Web Dashboard | ✅ |
| All-in-One deployment (embedded NATS + SQLite, no external deps) | ✅ |
| Telemetry export (VictoriaMetrics push) | ✅ |
| Multi-region / multi-cloud bridging | 🔜 |
| Smart DNS (internal service naming) | 🔜 |

---

## Quick Start

### Option A — All-in-One (No Kubernetes Required)

The all-in-one image bundles the control plane, embedded NATS, and SQLite into a single container. Ideal for evaluation and small deployments.

**Docker:**

```bash
docker run -d \
  --name wireflow \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 4222:4222 \
  -v wireflow-data:/app/data \
  ghcr.io/wireflowio/wireflowd:latest
```

Open the dashboard: [http://localhost:8080](http://localhost:8080)
Default credentials: `admin` / `changeme` (**change this immediately**)

**Docker Compose:**

```bash
curl -sSL https://raw.githubusercontent.com/wireflowio/wireflow/master/deploy/aio-compose.yml -o docker-compose.yml
docker compose up -d
```

---

### Option B — Kubernetes (Recommended for Production)

Requires `kubectl` pointed at an existing cluster. The quickstart script handles everything including k3d for local testing.

```bash
curl -sSL https://raw.githubusercontent.com/wireflowio/wireflow/master/hack/quickstart.sh | bash
```

The script will:
1. Verify Docker, k3d, and kubectl are present (installing missing tools automatically).
2. Check that ports **8080** (Dashboard / API) and **4222** (NATS signaling) are free.
3. Create a local k3d cluster and apply CRDs, RBAC, and Deployments.
4. Wait for the pod to become healthy.
5. Print a ready-to-use `wireflow up` command with the NATS address and initial token.

**Existing cluster (kustomize):**

```bash
kubectl apply -k https://github.com/wireflowio/wireflow/config/wireflow/overlays/all-in-one
```

---

## Connecting an Agent

All management commands below use `--signaling-url` to reach the embedded NATS server (default port 4222).

### 1. Create a workspace

```bash
wireflow workspace add dev \
  --display-name "Development" \
  --signaling-url nats://localhost:4222
```

```bash
# List all workspaces (shows namespace values used in subsequent commands)
wireflow workspace list --signaling-url nats://localhost:4222
```

### 2. Create an enrollment token

```bash
wireflow token create dev-team \
  -n <namespace> \
  --limit 10 \
  --expiry 168h \
  --signaling-url nats://localhost:4222
```

| Flag | Description |
|------|-------------|
| `-n` / `--namespace` | Workspace namespace (from `workspace list`) |
| `--limit` | Max agent connections (0 = unlimited) |
| `--expiry` | Token lifetime (e.g. `24h`, `168h`, omit = never) |

### 3. Start an agent

```bash
wireflow up --signaling-url nats://localhost:4222 --token <token>
```

Run as a container:

```bash
docker run -d \
  --name wf-agent \
  --restart unless-stopped \
  --privileged \
  --network host \
  ghcr.io/wireflowio/wireflow:latest \
  up --signaling-url nats://localhost:4222 --token <token>
```

### 4. Allow traffic between peers

Wireflow enforces a **default-deny** policy — agents can establish tunnels but cannot exchange traffic until a policy explicitly permits it. This prevents accidental exposure in multi-tenant environments.

**CLI — allow all traffic in a workspace (development / single-tenant):**

```bash
wireflow policy allow-all \
  -n <namespace> \
  --signaling-url nats://localhost:4222
```

**CLI — fine-grained policy:**

```bash
wireflow policy add my-policy \
  -n <namespace> \
  --action ALLOW \
  --desc "allow all peer traffic" \
  --signaling-url nats://localhost:4222
```

**Dashboard — visual policy editor:**

Navigate to `http://localhost:8080` → **Policies** → **Create Policy**.

You can define rules scoped to specific peers (by label), ports, and direction (ingress / egress).

**kubectl — apply a policy CRD directly:**

```yaml
apiVersion: wireflowcontroller.wireflow.run/v1alpha1
kind: WireflowPolicy
metadata:
  name: allow-all
  namespace: default
  labels:
    action: ALLOW
  annotations:
    description: "Full mesh — allow all peer traffic"
    policyTypes: "Ingress,Egress"
spec:
  action: ALLOW
  peerSelector: {}   # matches all peers in the namespace
  ingress: []        # empty = no port restriction
  egress: []
```

```bash
kubectl apply -f policy-allow-all.yaml
```

### 5. Verify connectivity

```bash
# List connected peers (Dashboard → Nodes)
# Check agent status
wireflow status
```

---

## CLI Reference

All commands accept `--signaling-url nats://<host>:4222` to target the control plane.

### Workspace

```bash
wireflow workspace add <slug> [--display-name <name>] [-n <namespace>]
wireflow workspace list
wireflow workspace remove <namespace>
```

### Token

```bash
wireflow token create <name> [-n <namespace>] [--limit <n>] [--expiry <duration>]
wireflow token list  [-n <namespace>]
wireflow token delete <token>
```

### Policy

```bash
wireflow policy allow-all -n <namespace>
wireflow policy add <name>  -n <namespace> [--action ALLOW|DENY] [--desc <text>]
wireflow policy list  -n <namespace>
wireflow policy remove <name> -n <namespace>
```

---

## Configuration Reference

The control plane is configured via a YAML file (default: `/etc/wireflow/wireflow.yaml`):

```yaml
app:
  listen: :8080
  name: "WireFlow"
  env: "production"
  init_admins:
    - username: "admin"
      password: "changeme"        # ⚠ Change before deploying

jwt:
  secret: "replace-with-random-secret"   # ⚠ Use a 32-byte random value
  expire_hours: 24

signaling-url: "nats://localhost:4222"   # Embedded NATS in all-in-one mode

database:
  dsn: "data/wireflow.db"                # SQLite (all-in-one)
  # dsn: "root:pass@tcp(mariadb:3306)/wireflow?charset=utf8mb4&parseTime=True"  # MariaDB
```

---

## Development

### Requirements

- Go 1.24+
- Docker 20.10+
- kubectl 1.20+ (for K8s features)

### Build from source

```bash
git clone https://github.com/wireflowio/wireflow.git
cd wireflow
make build-all
```

### Run locally (without Kubernetes)

```bash
# Start control plane in all-in-one mode
go run ./cmd/manager up --config config/wireflow/overlays/all-in-one/configmap.yaml

# Start an agent
go run ./cmd/wireflow up --signaling-url nats://localhost:4222 --token <token>
```

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a pull request.

<a href="https://github.com/wireflowio/wireflow/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=wireflowio/wireflow" />
</a>

---

## Disclaimer

This tool is intended for legitimate technical research, enterprise private networking, and compliant remote access scenarios only. Users are responsible for ensuring their use complies with all applicable local laws and regulations. The authors assume no liability for any misuse of this software.

## License

[Apache License 2.0](LICENSE)
