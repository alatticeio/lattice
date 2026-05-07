# Wireflow 商业化路线图

> 文档目的：记录从当前开发状态到可商业化上线所需完成的所有工作项，按优先级分阶段执行。
>
> 当前日期：2026-04-27


---

## 整体节奏

```
第一阶段  修复阻塞项          → 产品可以稳定交给外部用户使用
第二阶段  建立收费闭环        → 产品可以收钱
第三阶段  可交付 / 可分发     → 客户可以自助部署
第四阶段  信任与增长          → 降低获客成本，提升付费转化
```

---

## 第一阶段：修复阻塞项（约 1~2 周）

这些是上线前必须解决的功能缺陷，任何一项触发都会导致崩溃或明显的产品体验破洞。

### 1.1 消除运行时 panic ✅

~~`peerController.UpdateStatus` 和 `peerService.UpdateStatus` 两处都是 `panic("implement me")`~~

**已解决**：两处 panic 已替换为 no-op（返回 `nil`）。该方法目前无实际业务语义——节点在线状态由心跳驱动，无需手动写入，保留接口签名以维持兼容性。

---

### 1.2 实现节点管理操作（禁用 / 删除）✅

**已解决**：实现了禁用、启用、删除三个管理操作。

| 操作 | 含义 | 可逆 |
|------|------|------|
| **禁用**（Disable） | K8s annotation `wireflow.io/disabled=true`，节点保留但被标记踢出网络 | 是，可重新启用 |
| **删除**（Delete） | 永久删除 `WireflowPeer` CRD 及关联 ConfigMap | 否 |

后端新增路由：
- `PUT /api/v1/peers/:name/disable`
- `PUT /api/v1/peers/:name/enable`
- `DELETE /api/v1/peers/:name`

前端：操作菜单提供「禁用」「启用」「删除」选项，禁用/删除均有二次确认弹窗，被禁用节点列表显示 `disabled` 状态标签。

---

### 1.3 修复 Peering 连接页面

**问题**
`pages/manage/peers/index.vue` 中的连接列表是硬编码的假数据。这是前台可见页面，外部用户看到后会直接质疑产品真实性。

**解决方案**（二选一）
- **接入真实数据**：实现 Network Peering 功能的后端 API（见 `docs/design/network-peering.md`），前端调用真实接口。
- **临时隐藏**：如果 Peering 功能还未到发布阶段，从侧边栏导航中隐藏该菜单项，避免用户误入。

---

### 1.4 修复监控 API 鉴权漏洞 ✅

~~`/api/v1/monitor/topology` 端点没有挂载 Auth 中间件，任何人无需认证即可访问~~

**已解决**：`management/server/monitor.go` 中已在 `monitorRouter` 上统一添加 `middleware.AuthMiddleware()`，`/topology` 和 `/ws-snapshot` 两个端点现在都需要有效 Token 才能访问。

---

## 第二阶段：建立收费闭环（约 2~3 周）

产品功能稳定后，需要能收到钱。这是商业化最核心的一步。

### 2.1 设计定价方案

在做任何技术实现之前，先确定：

| 维度 | 建议初始方案 |
|------|------------|
| 定价模式 | 按工作空间节点数 + 按年订阅 |
| Community 版 | 免费，无节点数限制，但无监控/SSO/AI 功能 |
| Pro 版 | 付费，解锁监控、Dashboard、Dex SSO、AI 助手 |
| 私有部署 | 一次性 License 费用 + 年度维护费 |
| SaaS 托管 | 月付，按节点数阶梯计费（可后期推出） |

---

### 2.2 实现 License Key 校验机制

**当前问题**
Pro 功能只靠 `//go:build pro` 构建标签区分。客户拿到 Pro 二进制后可永久使用，无法控制：
- 授权到期
- 节点数量上限
- 授权绑定（防止一个 License 多处部署）

**推荐方案：签名 License JWT**

License 文件本质上是一个 JWT，由私钥签名，内容包含：

```json
{
  "customer": "Acme Corp",
  "plan": "pro",
  "max_nodes": 50,
  "expires_at": "2027-04-27T00:00:00Z",
  "features": ["monitoring", "sso", "ai"]
}
```

实现步骤：
1. 使用 Ed25519 生成签名密钥对，私钥离线保存，公钥内嵌到 Pro 二进制中。
2. 签发工具：写一个内部命令行工具 `cmd/licensegen`，输入客户信息输出 License 文件。
3. 运行时校验：Pro 构建启动时读取 `--license-file` 参数，验证签名 + 过期时间 + 节点数。
4. 超出配额时：API 返回 402 Payment Required，前端展示引导升级弹窗。

**需要新增的文件**
```
internal/license/
  license.go          # License 结构体 + 校验逻辑
  license_pro.go      # //go:build pro，从文件加载并校验
  license_community.go # //go:build !pro，返回 community 默认值
cmd/licensegen/
  main.go             # 内部签发工具
```

---

### 2.3 支付流程

**最小可行方案（手动）**
初期不需要自动化支付系统，手动流程成本最低：

1. 官网/文档放联系邮件或表单，客户填写需求
2. 发 Stripe Payment Link 或对公转账
3. 收到付款后用 `licensegen` 工具生成 License 文件发给客户

**后期自动化（当客户量上来后）**
- 集成 Stripe Checkout + Webhook
- 支付成功后自动签发 License 并发邮件

---

## 第三阶段：可交付 / 可分发（约 1~2 周）

客户付钱后需要能够自助部署，否则每次交付都要人工介入。

### 3.1 发布 Docker 镜像

需要发布以下镜像到 Docker Hub 或 GitHub Container Registry（ghcr.io）：

| 镜像 | 说明 |
|------|------|
| `wireflow/latticed:latest` | 管理控制面（社区版） |
| `wireflow/latticed-pro:latest` | 管理控制面（Pro 版） |
| `wireflow/wrrper:latest` | 中继服务器 |
| `wireflow/wireflow:latest` | 节点客户端（可选，通常直接发二进制） |

**构建流程**
- 在 GitHub Actions 中配置 CI/CD，tag 推送时自动构建并发布多架构镜像（`linux/amd64`, `linux/arm64`）。
- Community 版和 Pro 版用不同的构建参数（`-tags pro`）区分。

---

### 3.2 发布预编译客户端二进制

`wireflow` 节点客户端需要支持多平台安装，参考 Tailscale 的分发方式：

- **Linux**：`curl | sh` 一键安装脚本 + APT/YUM 仓库
- **macOS**：Homebrew tap 或 DMG 包
- **Windows**：MSI 安装包
- **GitHub Releases**：每个版本附带各平台二进制压缩包

---

### 3.3 发布 Helm Chart

K8s 部署是主要场景，需要提供生产可用的 Helm chart：

```
helm/
  wireflow/
    Chart.yaml
    values.yaml          # 包含 latticed、nats、victoria-metrics 配置
    templates/
      deployment.yaml
      service.yaml
      ingress.yaml
      configmap.yaml
      secret.yaml        # license key
      crd.yaml
      rbac.yaml
```

**values.yaml 关键配置项**
```yaml
latticed:
  image: wireflow/latticed-pro:latest
  license:
    secretName: wireflow-license  # K8s Secret 存放 license 文件
  config:
    signalingUrl: nats://nats:4222
    monitor:
      address: http://victoria-metrics:8428

nats:
  enabled: true   # 可选内置，也可指向外部 NATS

victoriaMetrics:
  enabled: true   # 可选内置，也可指向外部 VM
```

---

### 3.4 编写用户文档

当前 `docs/` 里是内部设计文档，需要面向用户的文档站。最小内容集：

| 文档 | 内容 |
|------|------|
| 快速开始 | 10 分钟从零跑起来的完整步骤（基于 Docker Compose） |
| K8s 部署指南 | 使用 Helm chart 部署到生产集群 |
| 节点接入指南 | 三个平台（Linux/macOS/Windows）的 `wireflow up` 教程 |
| 配置参考 | `wireflow.yaml` 所有配置项说明 |
| 监控配置 | VictoriaMetrics 集成、Dashboard 数据说明 |
| ACL 策略 | 如何编写和应用网络访问策略 |
| FAQ | 常见问题（NAT 穿透失败、节点离线等） |

工具建议：[Mintlify](https://mintlify.com) 或 [VitePress](https://vitepress.dev)，都可以用 Markdown 源文件快速搭建。

---

## 第四阶段：信任与增长（持续进行）

### 4.1 官网落地页

需要一个公开网站，回答三个问题：
- **是什么**：基于 WireGuard 的企业级零信任组网平台
- **解决什么**：跨云/跨机房/边缘节点的安全互联，替代复杂的 VPN 配置
- **怎么收费**：定价页，Community 免费 vs Pro 付费对比

### 4.2 压力测试 / 稳定性验证

正式对外销售前，建议完成：
- 50 节点 mesh 网络性能测试（延迟、吞吐量、控制面响应时间）
- 长时间运行稳定性测试（连续 72 小时，模拟节点上下线）
- K8s 控制面（latticed）重启后的恢复时间测试

### 4.3 安全自查清单

对于网络安全产品，安全性是核心信任基础：

- [ ] 所有管理 API 端点都有鉴权校验（重点检查上线新增的路由）
- [ ] WireGuard 私钥存储方式（K8s Secret 是否加密存储）
- [ ] NATS 信令通道是否支持 TLS
- [ ] SQL 注入/XSS 扫描（前端输入 + 后端查询）
- [ ] 依赖库漏洞扫描（`go audit` + `npm audit`）
- [ ] 敏感信息不出现在日志中（私钥、Token、密码）

### 4.4 开源策略

Community 版开源、Pro 版闭源是常见的商业模式（Open Core）：

- 将 Community 代码推送到 GitHub 公开仓库
- `//go:build pro` 的实现文件不开源，只开源 stub 文件
- 通过开源社区建立品牌认知，Pro 功能驱动商业收入

---

## 优先级总览

| # | 事项 | 优先级 | 预估工时 |
|---|------|--------|---------|
| 1.1 | ~~消除 UpdateStatus panic~~ ✅ | P0 | 已完成 |
| 1.2 | ~~实现节点禁用 / 删除 API~~ ✅ | P0 | 已完成 |
| 1.3 | 修复 Peering 页面（隐藏或实现） | P0 | 2h |
| 1.4 | ~~修复监控 API 鉴权~~ ✅ | P0 | 已完成 |
| 2.1 | 确定定价方案 | P1 | - |
| 2.2 | 实现 License Key 机制 | P1 | 3d |
| 2.3 | 建立支付流程 | P1 | 1d |
| 3.1 | 发布 Docker 镜像 + CI/CD | P1 | 2d |
| 3.2 | 发布客户端二进制 | P1 | 2d |
| 3.3 | 编写 Helm chart | P1 | 3d |
| 3.4 | 编写用户文档 | P1 | 1w |
| 4.1 | 官网落地页 | P2 | 1w |
| 4.2 | 压力测试 | P2 | 3d |
| 4.3 | 安全自查 | P2 | 3d |
| 4.4 | 开源策略执行 | P2 | 持续 |

---

## 最快路径

如果目标是**尽快找到第一批付费客户**，最小路径是：

```
完成第一阶段（修复 Bug）
    ↓
确定定价 + 手动 License 签发流程（2.1 + 2.2 + 2.3 简化版）
    ↓
发布 Docker 镜像 + 写一份 Getting Started 文档
    ↓
开始销售（直接联系目标客户、技术社区推广）
    ↓
根据客户反馈决定后续优先级
```

第一个付费客户不需要完美的官网和全自动化系统，需要的是**产品稳定可用**和**能解释清楚价值**。
