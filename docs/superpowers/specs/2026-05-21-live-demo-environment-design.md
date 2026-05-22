# Lattice 在线 Demo 环境设计

> 2026-05-21 | 设计阶段

## 目标

搭建一个公网可访问的 Lattice 在线试用环境，让用户无需本地安装即可：
1. 注册账号 → 创建 workspace → 生成 enrollment token
2. 通过 Web 终端在真实云主机上执行 `lattice join` / `lattice sandbox start`
3. 在 Dashboard 查看跨公网 overlay 拓扑、策略、审计日志

先用于内部开发验证（ICE 打洞、LRP 回退、sandbox 隔离），成熟后对外开放 Demo。

---

## 基础设施

### 主机规划（3 台轻量云服务器）

| 角色 | 区域 | 规格 | 用途 |
|------|------|------|------|
| **控制面 + Relay** | 北京 | 2C 4G, 40G SSD | latticed（内嵌 LRP relay）+ Dashboard + ttyd 网关 |
| **Agent 1** | 北京 | 1C 2G, 20G SSD | 与控制面同 region，测试同网段直连 |
| **Agent 2** | 上海 | 1C 2G, 20G SSD | 跨 region，测试公网 ICE 打洞 + LRP relay 回退 |

> 选用阿里云 ECS / AWS Lightsail，总成本约 $20-30/月。
> Agent 主机预装 `lattice` Pro 版二进制 + runsc（gVisor sandbox 模式需要）。

### 网络拓扑

```
                       公网
                        │
        ┌───────────────┼───────────────┐
        │               │               │
   ┌────▼──────┐   ┌────▼─────┐   ┌────▼──────┐
   │ 控制面     │   │ Agent 1  │   │ Agent 2   │
   │ 北京       │   │ 北京      │   │ 上海       │
   │ :443      │   │ :51820   │   │ :51820    │
   │ :4222     │   └────┬─────┘   └────┬──────┘
   │ :51820/udp│        │              │
   │ :7681-2   │        │  ICE 直连    │
   └───────────┘        └──────────────┘
         │                    │
         │  ┌─────────────────┘
         │  │  ICE 失败 → LRP relay (控制面内置)
         ▼  ▼
   ┌──────────────┐
   │ Dashboard    │  ← 用户浏览器
   │ Web 终端     │
   └──────────────┘
```

- 控制面开放 443（Dashboard + API）、4222（NATS）、7681-7682（ttyd）、51820/udp（relay 端口）
- Agent 主机开放 51820/udp（WireGuard + ICE）
- Agent1（北京）↔ Agent2（上海）：优先 ICE 直连，失败走 LRP relay（经控制面转发）
- Agent1（北京）↔ 控制面（北京）：同 region 直连

---

## 用户流程

```
用户浏览器
  │
  ├─ 1. 打开 https://demo.alattice.io
  │      → Dashboard 登录页
  │
  ├─ 2. 注册账号 → 登录
  │      默认管理员账号: admin / 自动生成密码
  │
  ├─ 3. 创建 workspace → 拿到 enrollment token
  │      Dashboard → WorkSpaces → Create
  │
  ├─ 4. 点击「Web 终端」→ 选择 agent 主机
  │      页面右侧弹出终端面板，自动连接到 agent 主机
  │
  ├─ 5. 在终端中执行:
  │      lattice join --token lt-xxx --server-url https://demo.alattice.io
  │      或
  │      lattice sandbox start --mode gvisor --name my-agent \
  │        --server-url https://demo.alattice.io --token lt-xxx \
  │        --agent-rootfs /opt/lattice/rootfs --agent-binary /usr/bin/ai-agent
  │
  ├─ 6. 刷新 Dashboard → 查看拓扑图、节点列表、sandbox agent 状态
  │
  └─ 7. 测试连通性: 在终端 ping 另一个 agent 的 overlay IP
```

---

## Web 终端方案

### ttyd + Nginx 反向代理

```
用户浏览器                    控制面主机
┌──────────┐                ┌──────────────────────────┐
│ Dashboard│                │  Nginx (:443)            │
│  iframe  │── wss ────────►│    /terminal/1 → :7681  │
│          │                │    /terminal/2 → :7682  │
└──────────┘                │                          │
                            │  ttyd :7681 → ssh agent1 │
                            │  ttyd :7682 → ssh agent2 │
                            └──────────────────────────┘
```

- 每个 agent 主机对应一个 ttyd 实例，通过 SSH 连接到 agent 主机
- ttyd 监听本地端口，Nginx 反向代理并添加 `/terminal/:id` 路径
- Dashboard 用 iframe 嵌入 `https://demo.alattice.io/terminal/:id`
- 用户登录 Dashboard 后才有权访问 Web 终端（Nginx auth_request 验证 JWT）

### 多用户隔离

| 用户 | Agent 主机 | 隔离方式 |
|------|-----------|---------|
| user-a | agent1 (北京) | 独立 Linux 用户 `user-a`，home 目录隔离 |
| user-b | agent2 (上海) | 独立 Linux 用户 `user-b` |

- 每个 Dashboard 用户映射到 agent 主机上的一个 Linux 用户
- ttyd 使用该 Linux 用户身份 SSH 登录
- `lattice` CLI 的配置和凭证在各自 home 目录下，互不干扰
- 首次登录时自动创建 Linux 用户（通过控制面的 user-create webhook）

---

## 部署方案

### Ansible 一键部署

```
deploy/demo/
├── inventory.yml          # 4 台主机 IP、角色、SSH 密钥
├── site.yml               # 主 playbook
├── roles/
│   ├── common/            # 基础环境：Docker, lattice CLI, runsc
│   ├── control-plane/     # latticed + Dashboard + Nginx + ttyd
│   └── agent/             # lattice CLI 预装 + rootfs 准备
└── group_vars/
    └── all.yml            # 域名、证书路径、版本号
```

```bash
# 部署
ansible-playbook -i inventory.yml site.yml

# 更新
ansible-playbook -i inventory.yml site.yml --tags update
```

### 控制面部署（Docker Compose）

```yaml
# docker-compose.control.yml
services:
  latticed:
    image: ghcr.io/alatticeio/latticed:latest
    ports: ["8080:8080", "4222:4222", "51820:51820/udp"]
    volumes:
      - ./data:/app/data
      - ./lattice.yaml:/etc/lattice/lattice.yaml
    restart: always

  nginx:
    image: nginx:alpine
    ports: ["443:443"]
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
      - ./certs:/etc/nginx/certs
    depends_on: [latticed]

  ttyd-agent1:
    image: tsl0922/ttyd:latest
    command: ttyd -p 7681 -c user:pass ssh agent1
    restart: always

  ttyd-agent2:
    image: tsl0922/ttyd:latest
    command: ttyd -p 7682 -c user:pass ssh agent2
    restart: always
```

### 前端改动（Dashboard 嵌入 Web 终端）

```
fronted/src/
├── components/terminal/
│   └── WebTerminal.vue     # iframe 嵌入 /terminal/:id
└── views/
    └── WorkspaceDetail.vue # 添加「打开终端」按钮
```

- `WebTerminal.vue`：响应式面板组件，从右侧滑出，iframe 加载 `/terminal/:id`
- 在 workspace 详情页、agent 详情页显示「打开终端」按钮
- 终端面板可通过拖拽调整宽度

---

## 安全

| 层级 | 措施 |
|------|------|
| 传输 | Nginx TLS 终止（Let's Encrypt），wss 加密 Web 终端流量 |
| 认证 | Dashboard JWT 验证；Nginx `auth_request` 校验终端访问 |
| 隔离 | 每个用户独立的 Linux 账户 + home 目录；agent 之间不共享凭证 |
| 限流 | Nginx rate limit：API 30 req/s，终端 5 conn/min |
| 审计 | latticed 审计日志；Nginx access log；ttyd 会话录制（可选） |
| 防火墙 | 仅开放 443、4222、51820/udp；SSH 仅内网 |

---

## 成本估算（月）

| 资源 | 规格 | 单价 | 数量 | 月费 |
|------|------|------|------|------|
| 控制面 | 2C 4G | ~$12 | 1 | $12 |
| Agent 节点 | 1C 2G | ~$6 | 2 | $12 |
| 公网流量 | 按量 | ~$0.08/GB | ~40GB | $3 |
| **合计** | | | | **~$27/月** |

> 用 AWS Lightsail 或阿里云轻量应用服务器可进一步降低成本。

---

## 待定

- [ ] 域名（demo.alattice.io 还是其他？）
- [ ] 云厂商选择（AWS / 阿里云 / 华为云？）
- [ ] Agent 主机是否开放注册（任意用户可 SSH？还是预设几个？）
- [ ] Web 终端会话录制（ttyd 支持 `--once` 单次会话模式）
