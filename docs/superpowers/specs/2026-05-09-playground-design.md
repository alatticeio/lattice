# Lattice Playground 体验体系设计

**日期**：2026-05-09
**背景**：国内无法做 SaaS，需要低摩擦的产品体验路径，让不同类型用户都能快速感受到 Lattice 的核心价值。

---

## 目标

让采购方、工程师、架构师都能在最短时间内体验到 Lattice 的完整功能链路，重点是 Dashboard 的丰富数据（监控、拓扑、策略、审计日志），而不是模拟网络底层细节。

---

## 用户画像与入口

| 入口 | 目标用户 | 摩擦 | 技术深度 |
|------|---------|------|---------|
| Demo 账号 | 采购方、PM、技术决策者 | 零（点击即进） | 中（看数据） |
| 自建 + 种子数据 | 工程师、架构师 | 低（1条命令） | 高（真实节点） |
| Sealos 一键部署 | 有云账号的企业用户 | 中（账号+点击） | 真实 |

文档站统一入口：`lattice.io/playground`

---

## 路径 1：Demo 账号（零摩擦）

### 概述

官方维护一套公开演示环境，任何人访问 `demo.lattice.io` 自动登录 demo 账号，无需注册。

### 基础设施

- 独立 `latticed` 实例，只读 API 模式
- 3-5 台真实 VPS 节点，分布国内不同地域（华北/华东/华南）
- 节点持续运行，产生真实流量和 WireGuard 握手数据
- 数据每 24 小时重置一次（定时任务）

### 权限

- 只读：可查看拓扑、流量、策略、审计日志
- 不可写：不能修改策略、不能踢节点、不能创建 Workspace

### Dashboard 展示内容

Demo 账号登录后可见：
- 实时网络拓扑图（真实节点，ICE 连接状态）
- 7 天流量趋势
- 策略列表 + 变更历史
- 操作审计日志（近 100 条）
- 节点详情（WireGuard 握手时间、RTT、字节数）

---

## 路径 2：自建 + 种子数据（动手路径）

### 快速安装

```bash
curl -sSL https://get.lattice.io | sh -s -- \
  --token <workspace-token> \
  --name my-node
```

脚本流程：
1. 检测 OS（Linux amd64/arm64，macOS）
2. 从国内 CDN 下载对应二进制
3. 写 systemd service / launchd plist
4. 启动 Agent，自动注册到 Workspace
5. 30 秒内节点出现在 Dashboard

### 种子数据注入

新 Workspace 创建时自动注入演示数据，让 Dashboard 第一次打开就有内容：

| 数据类型 | 数量 | 说明 |
|---------|------|------|
| 虚拟历史节点 | 8 个 | 部分 offline，更真实 |
| 历史流量数据 | 7 天 | 用于趋势图 |
| 策略规则 | 3 条 | 含变更历史 |
| 操作审计记录 | 20 条 | 模拟历史操作 |
| 已解决告警 | 2 条 | 展示告警功能 |

用户真实节点接入后，种子数据作为"历史背景"保留，可在设置页一键清除。

### 实现要点

```go
// internal/server/workspace/seed.go
type SeedOptions struct {
    VirtualNodes  int  // 默认 8
    HistoryDays   int  // 默认 7
    AuditEntries  int  // 默认 20
}

func InjectSeedData(ctx context.Context, workspaceID string, opts SeedOptions) error
```

- 种子数据存入正常数据表，加 `is_seed=true` 标记
- 前端拓扑图：虚拟节点用灰色虚线框渲染，真实节点用实线
- 设置页"清除演示数据"按钮：`DELETE WHERE is_seed=true`

---

## 路径 3：Sealos 一键部署

面向有云账号的企业用户，点击按钮在自己账号内拉起完整 Lattice 环境。

```yaml
# deploy/sealos/app.yaml
apiVersion: app.sealos.io/v1
kind: Template
metadata:
  name: lattice
spec:
  title: Lattice
  url: https://lattice.io
  gitRepo: https://github.com/alatticeio/lattice
  template:
    - latticed (all-in-one)
    - 预置种子数据
```

后续可扩展：阿里云应用中心、腾讯云应用市场。

---

## Dashboard 数据模块详情

```
Dashboard
├── 概览 (Overview)
│   ├── 节点健康度：在线/离线/告警 节点数
│   ├── 实时流量：入/出 带宽趋势（折线图，5s 刷新）
│   ├── 活跃连接数
│   └── 最近事件摘要（最新 5 条）
│
├── 网络拓扑 (Topology)
│   ├── 力导向图：节点 + 连接线（颜色表示连接质量）
│   ├── 点击节点：显示 IP、标签、WireGuard 公钥、延迟
│   └── 连接线：hover 显示协议（ICE直连/LRP中继）、RTT
│
├── 节点管理 (Nodes)
│   ├── 列表：名称、IP、标签、状态、最后在线时间
│   ├── 详情：WireGuard 握手时间、接收/发送字节数
│   └── 操作：添加标签、踢出网络、查看日志
│
├── 策略 (Policies)
│   ├── ACL 规则列表：源标签 → 目标标签 → 动作
│   ├── 变更历史：谁在什么时间改了什么（diff 视图）
│   └── 命中统计：每条规则的匹配次数（eBPF Pro）
│
├── 审计日志 (Audit)
│   ├── 操作记录：用户/Agent 的所有写操作
│   ├── 过滤：按时间、操作类型、操作人
│   └── 导出：CSV / JSON
│
└── 告警 (Alerts)
    ├── 节点离线告警
    ├── 策略冲突检测
    └── 异常流量提示
```

---

## 交付优先级

| 阶段 | 交付物 | 价值 |
|------|--------|------|
| P0 | Demo 账号 + 真实节点 | 立刻可用，无需后端开发 |
| P1 | 种子数据注入 | 新用户第一印象 |
| P2 | 安装脚本 + 国内 CDN | 降低动手摩擦 |
| P3 | Sealos 一键部署 | 企业用户入口 |

---

## 未采用的方案

- **交互式 Web Terminal**：WireGuard 需要内核模块，浏览器沙箱无法运行，排除。
- **docker-compose 多容器模拟 NAT**：复杂度高，体验价值不如 Dashboard 数据丰富性，排除。
