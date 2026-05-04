# Quick Start

Get a Lattice mesh network running in 10 minutes.

## 1. Deploy the Control Plane

### Docker (all-in-one)

```bash
docker run -d \
  --name lattice-k3s \
  --privileged \
  -p 8080:8080 \
  -p 4222:4222 \
  ghcr.io/alatticeio/lattice-k3s:latest
```

After ~30 seconds:
- Dashboard / API: `http://localhost:8080`
- NATS signaling: `nats://localhost:4222`

### Kubernetes (kustomize)

```bash
kubectl apply -k https://github.com/alatticeio/lattice/config/lattice/overlays/all-in-one
```

### Kubernetes (Helm)

```bash
helm repo add lattice https://alatticeio.github.io/lattice
helm install lattice lattice/lattice
```

## 2. Install the CLI

```bash
# macOS (Homebrew)
brew tap alatticeio/tap
brew install lattice

# Linux (Debian/Ubuntu)
curl -fsSL https://alatticeio.github.io/lattice/deb/Packages.gz -o /tmp/lattice-Packages.gz
echo "deb [trusted=yes] https://alatticeio.github.io/lattice/deb ./" | sudo tee /etc/apt/sources.list.d/lattice.list
sudo apt update && sudo apt install lattice

# Linux (RHEL/Fedora)
# Download from GitHub Releases
```

## 3. Configure the Agent

```bash
lattice init
```

Follow the prompts:
- Management server URL: `http://localhost:8080`
- Signaling URL: `nats://localhost:4222`
- Enrollment token: (create one via dashboard or CLI)

## 4. Connect

```bash
lattice up
```

## 5. Verify

```bash
lattice status
```

You should see your peer listed with a `connected` status.
