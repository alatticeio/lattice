# Agent Setup

The Lattice agent runs on any device to connect it to your mesh network.

## Interactive Setup

```bash
lattice init
```

This prompts for:
- **Management server URL** — The HTTP API endpoint (e.g. `http://lattice.example.com:8080`)
- **Signaling URL** — The NATS endpoint (e.g. `nats://lattice.example.com:4222`)
- **Enrollment token** — A token from your workspace
- **Relay URL** — (optional) TCP relay URL
- **Relay QUIC URL** — (optional) QUIC relay URL

Config is saved to `~/.lattice/lattice.yaml`.

## Connect

```bash
lattice up
```

After `lattice init`, no flags are needed. The NATS signaling URL is auto-discovered from the server via `/api/v1/discovery` — only `--server-url` and `--token` are required if you haven't run `lattice init`.

## Run as a Container

```bash
docker run -d \
  --name wf-agent \
  --restart unless-stopped \
  --privileged \
  --network host \
  -v ~/.lattice:/root/.lattice \
  ghcr.io/alatticeio/lattice:latest \
  up
```

## Status

```bash
lattice status
```

Shows WireGuard interface status, assigned IP, public key, and connected peers.

## Workspace Management

```bash
# List workspaces
lattice workspace list

# Create a workspace
lattice workspace add dev --display-name "Development"

# Remove a workspace
lattice workspace remove <namespace>
```

## Token Management

```bash
# Create an enrollment token
lattice token create my-token -n <namespace> --limit 10 --expiry 168h

# List tokens
lattice token list -n <namespace>

# Remove a token
lattice token remove <token>
```

## Policy Management

```bash
# Allow all traffic (development)
lattice policy allow-all -n <namespace>

# Fine-grained policy
lattice policy add my-policy \
  -n <namespace> \
  --action ALLOW \
  --desc "allow all peer traffic"

# List policies
lattice policy list -n <namespace>

# Remove a policy
lattice policy remove <name> -n <namespace>
```
