# Installation

## CLI (lattice agent)

### macOS

```bash
brew tap alatticeio/tap
brew install lattice
```

### Linux

**APT (Debian/Ubuntu):**

```bash
curl -fsSL https://alatticeio.github.io/lattice/deb/Packages.gz -o /tmp/lattice-Packages.gz
echo "deb [trusted=yes] https://alatticeio.github.io/lattice/deb ./" | sudo tee /etc/apt/sources.list.d/lattice.list
sudo apt update
sudo apt install lattice
sudo systemctl enable --now lattice
```

**YUM (RHEL/CentOS/Rocky/Fedora):**

Create `/etc/yum.repos.d/lattice.repo`:

```ini
[lattice]
name=Lattice
baseurl=https://alatticeio.github.io/lattice/rpm
enabled=1
gpgcheck=0
```

```bash
sudo yum install lattice
sudo systemctl enable --now lattice
```

### Binary Download

Download from [GitHub Releases](https://github.com/alatticeio/lattice/releases):

```bash
curl -sSL https://github.com/alatticeio/lattice/releases/latest/download/lattice_<version>_linux_amd64.tar.gz | tar xz
sudo mv lattice /usr/local/bin/
```

## Server (latticed)

### Docker

```bash
docker pull ghcr.io/alatticeio/latticed:latest
```

### Kubernetes (Helm)

```bash
helm repo add lattice https://alatticeio.github.io/lattice
helm install lattice lattice/lattice --namespace lattice-system --create-namespace
```

### Kubernetes (kustomize)

```bash
kubectl apply -k https://github.com/alatticeio/lattice/config/lattice/overlays/all-in-one
```
