# Helm 部署

## 前置条件

- Kubernetes 1.24+
- Helm 3.8+
- 节点可访问 `ghcr.io`（或提前拉取镜像）

---

## 安装 Chart

Chart 已发布到 GHCR OCI 仓库，无需克隆源码即可直接安装：

```bash
helm install lattice oci://ghcr.io/alatticeio/charts/lattice \
  --set config.jwt.secret="$(openssl rand -base64 32)" \
  --namespace lattice-system --create-namespace
```

安装完成后访问 Dashboard：

```bash
kubectl port-forward -n lattice-system svc/lattice 8080:8080
```

打开 `http://localhost:8080`，默认账号 `admin / changeme`。

> 如需指定版本，加上 `--version 0.x.x`。查看可用版本：
> ```bash
> helm show chart oci://ghcr.io/alatticeio/charts/lattice
> ```

---

## NATS 端口暴露

Agent 节点需要从集群外访问 NATS 信令端口（`4222`）。Chart 提供两种方式：

### 方式一：hostPort（默认，推荐用于 k3s）

`service.natsHostPort: true`（默认已开启），NATS 容器端口直接绑定到节点宿主机的 `4222`。

Agent 连接地址填写**节点 IP**：
```
nats://<节点IP>:4222
```

无需额外配置，适合 k3s / 单节点集群。

> **注意：** Pod 必须调度到有公网 IP（或 agent 可达 IP）的节点上，建议配合 `nodeSelector` 固定节点。

### 方式二：LoadBalancer Service

```yaml
service:
  type: LoadBalancer
  natsHostPort: false
```

Cloud 环境（EKS、GKE、AKS 等）会自动分配外部 IP。Agent 连接地址使用 Service 的 `EXTERNAL-IP`：

```bash
kubectl get svc -n lattice-system lattice -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

### 方式三：Traefik IngressRouteTCP（Traefik 用户）

需要 Traefik 提前配置名为 `nats` 的 TCP entrypoint（监听 `4222`）：

```yaml
# values.yaml
service:
  natsHostPort: false
ingressRouteTCP:
  enabled: true
  entrypoint: nats   # 与 Traefik entrypoint 名称一致
```

---

## 生产安装

### 1. 准备 values 文件

```yaml
ingress:
  enabled: true
  className: "nginx"
  host: lattice.example.com      # 替换为你的域名
  tls:
    enabled: true
    secretName: lattice-tls      # cert-manager 自动签发或手动创建

service:
  type: LoadBalancer
  natsHostPort: false            # 生产环境建议用 LoadBalancer

config:
  signalingUrl: "nats://nats.lattice-system.svc.cluster.local:4222"  # 外部 NATS
  stunUrl: "stun.alattice.io:3478"

extraEnv:
  - name: APP_DATABASE_DSN
    value: "user:pass@tcp(mysql:3306)/lattice?charset=utf8mb4&parseTime=True"
```

### 2. 安装

```bash
helm install lattice oci://ghcr.io/alatticeio/charts/lattice \
  -f my-values.yaml \
  --set config.jwt.secret="$(openssl rand -base64 32)" \
  --namespace lattice-system --create-namespace
```

> JWT secret 建议通过 `--set` 传入而不是写进 values 文件，避免明文提交到 Git。

---

## STUN 服务（coturn）

默认使用托管的 `stun.alattice.io:3478`，无需额外配置。

如果部署环境无法访问外部网络，可启用内置 coturn：

```bash
helm install lattice oci://ghcr.io/alatticeio/charts/lattice \
  --set config.jwt.secret="$(openssl rand -base64 32)" \
  --set coturn.enabled=true \
  --set config.stunUrl="<节点公网IP>:3478" \
  --namespace lattice-system --create-namespace
```

coturn 以 `--stun-only` 模式运行（纯 STUN，不做 TURN relay），通过 `hostPort: 3478/UDP` 绑定到节点的公网 IP。

**前提：**
- Pod 所在节点有公网 IP
- 安全组 / 防火墙开放 `3478/UDP`

---

## 配置参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `image.repository` | 镜像地址 | `ghcr.io/alatticeio/latticed` |
| `image.tag` | 镜像 tag | `latest` |
| `replicaCount` | 副本数 | `1` |
| `config.jwt.secret` | JWT 签名密钥（必填） | `""` |
| `config.jwt.expireHours` | Token 有效期（小时） | `24` |
| `config.signalingUrl` | 外部 NATS 地址；为空时使用 Pod 内嵌 NATS | `""` |
| `config.stunUrl` | STUN 服务地址 | `stun.alattice.io:3478` |
| `config.database.dsn` | SQLite 路径；设置 `APP_DATABASE_DSN` 环境变量可切换 MySQL | `data/lattice.db` |
| `service.type` | Service 类型 | `LoadBalancer` |
| `service.natsHostPort` | 将 NATS 4222 绑定到宿主机 hostPort（k3s 推荐） | `true` |
| `ingressRouteTCP.enabled` | 启用 Traefik IngressRouteTCP 暴露 NATS | `false` |
| `ingressRouteTCP.entrypoint` | Traefik TCP entrypoint 名称 | `nats` |
| `persistence.enabled` | 是否挂载 PVC（SQLite 模式必须开启） | `true` |
| `persistence.size` | 存储大小 | `10Gi` |
| `persistence.storageClass` | StorageClass；为空时使用集群默认 | `""` |
| `ingress.enabled` | 是否启用 Ingress | `false` |
| `ingress.className` | Ingress class | `""` |
| `ingress.host` | 域名 | `lattice.example.com` |
| `ingress.tls.enabled` | 是否启用 TLS | `false` |
| `ingress.tls.secretName` | TLS secret 名称 | `""` |
| `coturn.enabled` | 是否部署内置 STUN 服务 | `false` |
| `license.enabled` | 是否挂载 Pro 授权文件 | `false` |
| `license.fileContents` | Pro license JWT 内容 | `""` |
| `extraEnv` | 追加环境变量（如 `APP_DATABASE_DSN`） | `[]` |

---

## 升级

```bash
helm upgrade lattice oci://ghcr.io/alatticeio/charts/lattice \
  -f my-values.yaml \
  --set config.jwt.secret="<your-secret>" \
  --namespace lattice-system
```

## 卸载

```bash
helm uninstall lattice -n lattice-system
```

> CRD 不会随 `helm uninstall` 删除，如需清理：
> ```bash
> kubectl delete crd -l app.kubernetes.io/managed-by=Helm
> ```

---

## 常见问题

**Pod 起不来，提示 jwt.secret is required**

`config.jwt.secret` 未设置或为空。通过 `--set config.jwt.secret="..."` 传入，或在 values 文件中填写。生成随机密钥：

```bash
openssl rand -base64 32
```

**Agent 无法连接信令（4222 端口不可达）**

- **hostPort 模式（默认）**：确认 agent 使用的是 Pod 所在**节点 IP**，而非 Service ClusterIP 或 DNS 名称；检查节点防火墙是否放行 `4222/TCP`。
- **LoadBalancer 模式**：等待 `EXTERNAL-IP` 分配完成（`kubectl get svc -n lattice-system`），确认云安全组开放 `4222/TCP`。
- **Traefik IngressRouteTCP**：确认 Traefik 已配置 `nats` entrypoint，且 `ingressRouteTCP.entrypoint` 与之一致。

**agents 使用的 signalingUrl 怎么填**

Agent 配置文件或启动参数中填写可从外部访问的 NATS 地址，格式：

```
nats://<外部IP或域名>:4222
```

也可通过 Dashboard → Platform Settings → NATS Signaling URL 统一配置，Agent 会在启动时自动从 `/api/v1/discovery` 拉取。

**coturn 无法穿透**

确认 `config.stunUrl` 设置为节点实际公网 IP，且 `3478/UDP` 在安全组中已放行。
