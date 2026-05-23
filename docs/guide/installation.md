# 安装

## 一键安装（推荐）

自动探测操作系统和 CPU 架构，从 GitHub Releases 拉取对应二进制包：

```bash
curl -fsSL https://docs.alattice.io/install.sh | bash
```

支持 Linux（amd64 / arm64）和 macOS（amd64 / Apple Silicon）。安装完成后运行：

```bash
lattice --version
```

**安装 latticed（All-in-One 控制面，仅 Linux）：**

```bash
curl -fsSL https://docs.alattice.io/install.sh | BINARY=latticed bash
```

**安装指定版本：**

```bash
curl -fsSL https://docs.alattice.io/install.sh | TAG=v0.1.0-alpha bash
```

**自定义安装目录：**

```bash
curl -fsSL https://docs.alattice.io/install.sh | INSTALL_DIR=~/.local/bin bash
```

---

## 手动安装

### macOS（Homebrew）

```bash
brew tap alatticeio/tap
brew install lattice
```

### 二进制下载

从 [GitHub Releases](https://github.com/alatticeio/lattice/releases) 下载对应平台的包，解压后移动到 PATH：

```bash
# Linux amd64
curl -fsSL https://github.com/alatticeio/lattice/releases/latest/download/lattice_linux_amd64.tar.gz | tar xz
sudo mv lattice /usr/local/bin/

# macOS Apple Silicon
curl -fsSL https://github.com/alatticeio/lattice/releases/latest/download/lattice_darwin_arm64.tar.gz | tar xz
sudo mv lattice /usr/local/bin/
```

---

## 部署控制面（latticed）

### Docker

```bash
docker run -d \
  --name latticed \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 4222:4222 \
  -v lattice-data:/app/data \
  -e LATTICE_JWT_SECRET="$(openssl rand -base64 32)" \
  ghcr.io/alatticeio/latticed:latest
```

### Kubernetes（Helm）

```bash
helm install lattice ./deploy/charts/lattice \
  --set config.jwt.secret="$(openssl rand -base64 32)" \
  --namespace lattice-system --create-namespace
```

### Kubernetes（kustomize）

```bash
kubectl apply -k config/lattice/overlays/all-in-one
```

详细部署说明见 [Helm 部署](../deploy/helm) 和 [Kubernetes 部署](../deploy/k8s-operator)。
