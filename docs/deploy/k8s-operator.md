# Kubernetes 部署

Lattice 提供基于 Kustomize 的 K8s 部署配置，位于仓库 `config/lattice/` 目录。

---

## 前置条件

- Kubernetes 1.24+
- kubectl + kustomize（kubectl 1.14+ 内置）
- 集群节点可拉取 `ghcr.io` 镜像

---

## 部署模式对比

| 模式 | 适用场景 | 组件 |
|------|----------|------|
| **all-in-one** | 单节点、快速试用、自托管 | latticed（内嵌 NATS + SQLite）|
| **production** | 高可用、生产 | base + 独立 NATS + MariaDB + Dex |
| **dev** | 开发调试 | base + 独立 NATS + MariaDB + Dex，使用 dev 镜像 |

---

## All-in-One 部署

NATS 和 SQLite 内嵌在 `latticed` 单个 Pod 里，无需额外依赖。

### 1. 修改配置

编辑 `config/lattice/overlays/all-in-one/configmap.yaml`，至少替换以下两项：

```yaml
jwt:
  secret: "replace-with-random-secret"   # ⚠ 替换为随机值：openssl rand -base64 32

app:
  init_admins:
    - username: "admin"
      password: "changeme"               # ⚠ 替换为强密码
```

如需接入 AI 功能，同时填写：

```yaml
ai:
  enabled: true
  provider: anthropic        # anthropic / deepseek / openai
  api-key: "YOUR_API_KEY"    # 建议通过 K8s Secret + 环境变量注入
```

### 2. 部署

```bash
kubectl apply -k config/lattice/overlays/all-in-one
```

这会创建：
- `lattice-system` namespace
- `latticed` Deployment（含内嵌 NATS）
- API Service（8080）+ NATS Service（4222）
- PVC（SQLite 持久化）
- RBAC 所需的 ServiceAccount / ClusterRole / ClusterRoleBinding
- coturn STUN 服务（默认包含，用于 ICE NAT 穿透）
- 默认 LatticeGlobalIPPool（`10.100.0.0/16`）

### 3. 验证

```bash
kubectl get pods -n lattice-system
kubectl get latticenetworks
```

Pod 变为 `Running` 后访问 Dashboard：

```bash
kubectl port-forward -n lattice-system svc/lattice-api-service 8080:8080
```

打开 `http://localhost:8080`。

---

## Coturn（STUN 服务）

All-in-one overlay 默认包含内置 coturn，以 `--stun-only` 模式运行，通过 `hostPort: 3478/UDP` 绑定到节点公网 IP。

**前提：**
- Pod 所在节点有公网 IP
- 安全组 / 防火墙开放 `3478/UDP`

如果使用外部 STUN 服务，移除 coturn component 并修改 `configmap.yaml`：

```yaml
# kustomization.yaml 中删除：
components:
  - ../../components/coturn   # 删除这行

# configmap.yaml 中修改：
stun-url: "stun.alattice.io:3478"   # 改为外部 STUN 地址
```

---

## 切换到 MySQL/MariaDB

修改 `configmap.yaml` 中的 `database.dsn`：

```yaml
database:
  dsn: "root:pass@tcp(mariadb.lattice-system.svc:3306)/lattice?charset=utf8mb4&parseTime=True"
```

建议通过 K8s Secret 注入，避免密码写入 ConfigMap：

```yaml
# 创建 Secret
kubectl create secret generic lattice-db-secret \
  --from-literal=dsn="root:pass@tcp(mariadb:3306)/lattice?charset=utf8mb4&parseTime=True" \
  -n lattice-system

# deployment.yaml 中添加环境变量
env:
  - name: LATTICE_DATABASE_DSN
    valueFrom:
      secretKeyRef:
        name: lattice-db-secret
        key: dsn
```

---

## 生产部署（分离式组件）

生产 overlay 将 NATS、MariaDB、Dex（SSO）拆分为独立组件：

```bash
kubectl apply -k config/lattice/overlays/production
```

包含组件：
- `base`：manager + CRD + RBAC
- `components/nats`：独立 NATS 集群
- `components/database`：MariaDB
- `components/dex`：Dex SSO

生产 overlay 还包含亲和性调度（`podAntiAffinity`）将副本分散到不同节点。

---

## 更新镜像

修改 `kustomization.yaml` 中的 `images` 字段指定新版本：

```yaml
images:
  - name: ghcr.io/alatticeio/latticed
    newTag: v0.2.0
```

然后重新 apply：

```bash
kubectl apply -k config/lattice/overlays/all-in-one
```

---

## 查看 CRD

Lattice 安装后会注册以下 CRD：

| CRD | 用途 |
|-----|------|
| `latticenetworks.alattice.io` | 定义一个 overlay 网络（workspace） |
| `latticepeers.alattice.io` | 网络中的节点（agent 注册后自动创建） |
| `latticepolicies.alattice.io` | 网络访问控制策略 |
| `latticeglobalippools.alattice.io` | 全局 IP 地址池（默认 `10.100.0.0/16`） |

```bash
kubectl get latticenetworks -A
kubectl get latticepeers -A
kubectl get latticepolicies -A
```

---

## 卸载

```bash
kubectl delete -k config/lattice/overlays/all-in-one
```

CRD 不会自动删除，如需清理：

```bash
kubectl delete crd \
  latticenetworks.alattice.io \
  latticepeers.alattice.io \
  latticepolicies.alattice.io \
  latticeglobalippools.alattice.io
```

> ⚠ 删除 CRD 会同时删除所有相关资源对象，操作前请确认。
