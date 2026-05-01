# Lattice-K3s: All-in-One 单容器部署

在单个 Docker 容器中运行 K3s（轻量级 K8s）+ Lattice 控制面，实现"一条命令启动完整集群"。

## 架构

```
┌─────────────────────────────────────┐
│  docker run lattice-k3s            │
│                                     │
│  ┌─────────────┐   ┌──────────────┐│
│  │  K3s         │   │  latticed    ││
│  │  (背景进程)   │   │  (前台进程)   ││
│  │             │   │              ││
│  │  K8s API    │   │  REST API   ││  :8080
│  │  Server     │◄──│  Web UI     ││
│  │  + CRDs     │   │  NATS       ││  :4222
│  │             │   │  SQLite     ││
│  └─────────────┘   └──────────────┘│
│                                     │
│  /var/lib/rancher/k3s/server/      │
│    manifests/crds/  ← 12 个 CRD   │
│                                     │
└─────────────────────────────────────┘
```

- **K3s**: 提供 K8s API Server 用于 CRD 管理（`--disable traefik --disable servicelb`）
- **latticed**: 作为二进制直接运行（非 Pod），通过 KUBECONFIG 连接 K3s API
- **CRDs**: 放在 `/var/lib/rancher/k3s/server/manifests/` 目录，K3s 启动时自动 apply
- **端口**: 8080（Dashboard/API）、4222（NATS 信令）

## 构建

```bash
# 从项目根目录构建
docker build -f deploy/k3s/Dockerfile -t lattice-k3s:latest .

# 使用 buildkit 缓存加速
DOCKER_BUILDKIT=1 docker build -f deploy/k3s/Dockerfile -t lattice-k3s:latest .

# 指定构建参数（可选）
docker build \
  --build-arg BUILD_TAGS=pro \
  -f deploy/k3s/Dockerfile \
  -t lattice-k3s:latest \
  .
```

### GitHub Actions 自动构建

项目已配置 GitHub Actions 工作流（`.github/workflows/build-k3s-image.yml`）：

| 触发条件 | Tag |
|---------|-----|
| Push 到 master 且涉及 k3s/CRD/核心代码变更 | `latest` |
| PR 到 master | `pr-<number>` |

镜像推送至 `ghcr.io/alatticeio/lattice-k3s`。

## 使用

### 快速启动

```bash
docker run -d \
  --name lattice \
  --privileged \
  -p 8080:8080 \
  -p 4222:4222 \
  -v lattice-data:/app/data \
  ghcr.io/alatticeio/lattice-k3s:latest
```

启动后（约 15-30 秒）打开浏览器访问 `http://localhost:8080`，使用默认管理员账号登录。

### 自定义管理员密码

```bash
docker run -d \
  --name lattice \
  --privileged \
  -p 8080:8080 \
  -p 4222:4222 \
  -e LATTICE_ADMIN_PASS="my-secure-password" \
  -e LATTICE_JWT_SECRET="$(openssl rand -hex 32)" \
  ghcr.io/alatticeio/lattice-k3s:latest
```

### 持久化数据

SQLite 数据库默认存储在 `/app/data/lattice.db`。挂载卷可持久化：

```bash
docker run -d \
  --name lattice \
  --privileged \
  -p 8080:8080 \
  -p 4222:4222 \
  -v /path/to/data:/app/data \
  ghcr.io/alatticeio/lattice-k3s:latest
```

### 连接 Agent

```bash
# 在有 WireGuard 的设备上
lattice up --signaling-url nats://<容器主机IP>:4222 --token <token>
```

Token 在 Dashboard 中创建，或通过 CLI：

```bash
docker exec lattice lattice token create my-token \
  --signaling-url nats://localhost:4222 \
  -n default --limit 10
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LATTICE_ADMIN_USER` | `admin` | 管理员用户名 |
| `LATTICE_ADMIN_PASS` | `changeme` | 管理员密码（首次启动后生效） |
| `LATTICE_JWT_SECRET` | 自动生成 | JWT 签名密钥 |
| `LATTICE_DATA_DIR` | `/app/data` | SQLite 数据库目录 |
| `LATTICE_CONFIG_DIR` | `/etc/lattice` | 配置文件目录 |

## 用户场景

| 用户类型 | 价值 |
|---------|------|
| **NAS 用户** (群晖/极空间) | 一条命令在 NAS 上起控制面，远程设备通过 `lattice up` 连回来，不需要公网 IP |
| **企业 K8s 团队** | 已有 K8s 集群，直接用 `kubectl apply -k` 部署，该镜像主要用于测试/演示 |
| **开发者** | 快速拉起完整的 Lattice 环境用于开发调试 |

## 注意事项

1. **特权模式**: K3s 需要 `--privileged` 权限，这是硬性要求
2. **资源开销**: 容器内存建议不低于 512MB（K3s ~200MB + latticed ~50MB）
3. **启动时间**: 首次启动约 15-30 秒（K3s 初始化 + CRD 部署）
4. **平台兼容**: K3s 需要 Linux 内核 cgroup v2 支持——Docker Desktop（Linux）、OrbStack、Linux 宿主机均支持；macOS 上需通过 Docker 虚拟机运行

## 文件说明

| 文件 | 说明 |
|------|------|
| `deploy/k3s/Dockerfile` | 多阶段构建：编译 latticed → 打包到 K3s 镜像 |
| `deploy/k3s/start.sh` | 容器入口：启 K3s → 等 API 就绪 → 创建 CRD 资源 → 生成配置 → 启 latticed |
| `.github/workflows/build-k3s-image.yml` | GitHub Actions 自动构建 + 冒烟测试 |

## 与现有部署方式对比

| 方式 | 命令 | 适用场景 |
|------|------|---------|
| **lattice-k3s 容器** | `docker run lattice-k3s` | 单机、NAS、开发测试 |
| **kubectl apply** | `kubectl apply -k config/lattice/overlays/all-in-one` | 已有 K8s 集群 |
| **Helm chart** | `helm install lattice ...` | 生产环境、GitOps |
