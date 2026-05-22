# Helm 部署

## 前置条件

- Kubernetes 1.24+
- Helm 3.8+
- 节点可访问 `ghcr.io`（或提前拉取镜像）

---

## 快速安装（开发/试用）

使用仓库内的 chart 直接安装，SQLite 存储，单副本：

```bash
helm install lattice ./deploy/charts/lattice \
  --set config.jwt.secret="$(openssl rand -base64 32)" \
  --namespace lattice-system --create-namespace
```

安装完成后访问 Dashboard：

```bash
kubectl port-forward -n lattice-system svc/lattice 8080:8080
```

打开 `http://localhost:8080`，默认账号 `admin / changeme`。

---

## 生产安装

### 1. 准备 values 文件

复制生产模板并按需修改：

```bash
cp deploy/charts/lattice/values.prod.yaml my-values.yaml
```

关键字段：

```yaml
ingress:
  enabled: true
  className: "nginx"
  host: lattice.example.com      # 替换为你的域名
  tls:
    enabled: true
    secretName: lattice-tls      # cert-manager 自动签发或手动创建

config:
  signalingUrl: "nats://nats.lattice-system.svc.cluster.local:4222"  # 外部 NATS
  stunUrl: "stun.alattice.io:3478"   # 使用托管 STUN（默认），或填自建地址

extraEnv:
  - name: APP_DATABASE_DSN
    value: "user:pass@tcp(mysql:3306)/lattice?charset=utf8mb4&parseTime=True"
```

### 2. 安装

```bash
helm install lattice ./deploy/charts/lattice \
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
helm install lattice ./deploy/charts/lattice \
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
helm upgrade lattice ./deploy/charts/lattice \
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

**agents 无法连接信令**

检查 `config.signalingUrl`：
- 单节点模式（默认）：NATS 内嵌在 Pod 里，agent 需要能访问到 Pod 的 4222 端口
- 生产模式：填写外部 NATS 地址，确保 agent 网络可达

**coturn 无法穿透**

确认 `config.stunUrl` 设置为节点实际公网 IP，且 `3478/UDP` 在安全组中已放行。
