# All-in-One 部署

`latticed` 将 NATS + SQLite + API + Dashboard 打包为单个进程，无需任何外部依赖，适合单节点部署、自托管场景。

---

## Docker

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

- `8080`：Dashboard + REST API
- `4222`：NATS 信令（agent 连接用）
- `lattice-data`：持久化 SQLite 数据

启动后访问 `http://<host>:8080`，默认账号 `admin / changeme`。

---

## Docker Compose

```yaml
services:
  latticed:
    image: ghcr.io/alatticeio/latticed:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "4222:4222"
    volumes:
      - lattice-data:/app/data
      - ./lattice.yaml:/etc/lattice/lattice.yaml
    environment:
      - LATTICE_JWT_SECRET=${LATTICE_JWT_SECRET}

volumes:
  lattice-data:
```

在同目录创建 `.env` 文件：

```bash
LATTICE_JWT_SECRET=$(openssl rand -base64 32)
```

---

## 配置文件

配置文件默认路径为 `/etc/lattice/lattice.yaml`，也可通过 `--config` 参数指定路径。

完整示例：

```yaml
app:
  listen: ":8080"
  name: "Lattice"
  env: "production"
  playground: false          # true 时解锁全部 Pro 功能（无需 license，适合试用）
  initAdmins:
    - username: "admin"
      password: "changeme"   # ⚠ 首次启动后请立即修改

jwt:
  secret: "replace-with-random-secret"   # ⚠ 必填，建议 openssl rand -base64 32 生成
  expire_hours: 24

database:
  dsn: "data/lattice.db"     # SQLite（默认）
  # dsn: "root:pass@tcp(mysql:3306)/lattice?charset=utf8mb4&parseTime=True"  # MySQL/MariaDB

signaling-url: ""            # 为空时使用进程内嵌 NATS（默认）
stun-url: "stun.alattice.io:3478"   # STUN 服务地址

metrics:
  port: 9685                 # Prometheus metrics 端口，不需要可删除
```

### 通过环境变量覆盖

所有配置项均可通过环境变量覆盖，格式为 `LATTICE_<KEY>` （大写，`.` 和 `-` 替换为 `_`）：

| 环境变量 | 对应配置 |
|----------|----------|
| `LATTICE_JWT_SECRET` | `jwt.secret` |
| `LATTICE_DATABASE_DSN` | `database.dsn` |
| `LATTICE_SIGNALING_URL` | `signaling-url` |
| `LATTICE_SERVER_URL` | agent 连接控制面的地址 |

---

## 使用 MySQL/MariaDB

默认使用 SQLite，无需额外配置。切换到 MySQL：

```yaml
database:
  dsn: "user:pass@tcp(mysql-host:3306)/lattice?charset=utf8mb4&parseTime=True"
```

或通过环境变量传入（推荐，避免密码写入配置文件）：

```bash
docker run -d \
  -e LATTICE_DATABASE_DSN="user:pass@tcp(mysql-host:3306)/lattice?charset=utf8mb4&parseTime=True" \
  ghcr.io/alatticeio/latticed:latest
```

---

## 二进制直接运行

从 [Releases](https://github.com/alatticeio/lattice/releases) 下载 `latticed` 二进制：

```bash
# 下载
curl -L https://github.com/alatticeio/lattice/releases/latest/download/latticed_linux_amd64.tar.gz | tar xz
chmod +x latticed

# 创建配置
mkdir -p /etc/lattice
cat > /etc/lattice/lattice.yaml <<EOF
app:
  listen: ":8080"
jwt:
  secret: "$(openssl rand -base64 32)"
EOF

# 运行
./latticed serve
```

### systemd 服务

```ini
[Unit]
Description=Lattice All-in-One Control Plane
After=network.target

[Service]
ExecStart=/usr/bin/latticed serve
Restart=on-failure
RestartSec=5s
Environment=LATTICE_JWT_SECRET=<your-secret>

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable --now latticed
```

---

## 常见问题

**启动报错 `jwt.secret is required` 或 token 验证失败**

`jwt.secret` 未配置或为空。通过配置文件或 `LATTICE_JWT_SECRET` 环境变量设置。

**agent 无法连接信令**

确认 agent 的 `--server-url` 指向 `http://<latticed-host>:8080`，agent 会自动通过 `/api/v1/discovery` 获取 NATS 地址。也可以直接用 `LATTICE_SIGNALING_URL=nats://<host>:4222` 覆盖。

**数据持久化**

SQLite 文件默认写入进程当前目录的 `data/lattice.db`。Docker 部署必须挂载 volume，否则容器重启后数据丢失。
