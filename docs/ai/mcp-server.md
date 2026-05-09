---
title: MCP Server + AI ChatOps
---

# MCP Server — User Guide

让 Claude Desktop / Claude Code 直接管理 Lattice 网络。

## What is MCP?

Model Context Protocol (MCP) 是 Anthropic 发布的开放协议。Lattice MCP Server 实现了这个协议，让 AI 助手可以直接操作你的 WireGuard 网络——创建 Peer、管理策略、查看拓扑，全部通过自然语言。

## Quick Start (5 分钟)

### 1. 启动 Lattice 控制平面

```bash
latticed start
```

### 2. 登录并获取 workspace ID

```bash
# 登录
curl -s http://localhost:8080/api/v1/users/login \
  -X POST -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

Token 自动保存到 `~/.lattice/lattice.yaml`。

### 3. 找到 workspace ID

在 Dashboard UI → 选择 workspace，查看 URL 中的 ID。或：

```bash
curl -s http://localhost:8080/api/v1/workspaces/list \
  -H "Authorization: Bearer $(grep auth-token ~/.lattice/lattice.yaml | awk '{print $2}')"
```

### 4. 配置 Claude Desktop

编辑 `~/.config/claude/claude_desktop_config.json`（macOS）：

```json
{
  "mcpServers": {
    "lattice": {
      "command": "lattice-mcp",
      "args": ["--workspace", "YOUR_WORKSPACE_ID"]
    }
  }
}
```

或 Claude Code (`~/.claude/claude.json`)：

```json
{
  "mcpServers": {
    "lattice": {
      "command": "lattice-mcp",
      "args": ["--workspace", "YOUR_WORKSPACE_ID"]
    }
  }
}
```

### 5. 重启 Claude Desktop

在对话里试试：

> "列出我网络里所有的 Peer"

Claude 会自动调用 MCP 工具返回结果。

---

## Available Tools (14)

### Read

| Tool | Description |
|------|-------------|
| `list_peers` | 列出所有 Peer，含在线状态、IP、标签 |
| `list_policies` | 列出所有访问控制策略 |
| `list_networks` | 列出所有网络及 CIDR |
| `check_connectivity` | 检查两个 Peer 间是否有策略允许通信 |

### Write (needs approval)

| Tool | Description |
|------|-------------|
| `create_peer` | 创建新 Peer 节点 |
| `delete_peer` | 删除 Peer 节点 |
| `create_policy` | 创建访问控制策略 |
| `delete_policy` | 删除策略 |

### Pro (needs `ai.enabled=true` + LLM API Key)

| Tool | Description |
|------|-------------|
| `plan_network_change` | 自然语言意图 → 变更计划预览 |
| `apply_network_change` | 执行变更计划 |
| `list_snapshots` | 网络状态历史快照 |
| `get_snapshot` | 获取指定快照完整状态 |
| `diff_snapshots` | 对比两个快照的变更 |
| `check_connectivity_at` | 在历史快照状态检查连通性 |

---

## Local Dev Verification

```bash
# 1. Start latticed
latticed start

# 2. Build MCP binary
make build-mcp

# 3. Login
TOKEN=$(curl -s http://localhost:8080/api/v1/users/login \
  -X POST -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"123456"}' \
  | jq -r '.data.token')

# 4. Update config
sed -i '' "s|auth-token: .*|auth-token: $TOKEN|" ~/.lattice/lattice.yaml

# 5. Test via stdio (Ctrl+D to exit)
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./bin/lattice-mcp --workspace YOUR_WORKSPACE_ID
```

Expected: returns 14 tools.

---

## AI Chat (Pro)

```yaml
# ~/.lattice/lattice.yaml
ai:
  enabled: true
  api-key: "sk-..."         # Anthropic / DeepSeek / OpenAI
  provider: anthropic
  max-tool-calls: 5
```

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `tools/list` returns 404 | `latticed` not running or `server-url` wrong in config |
| "workspace not found" | `--workspace` arg is not a valid workspace ID |
| Claude Desktop can't find `lattice-mcp` | Use absolute path in config, or add to PATH |
| `ai.enabled=true` but tools still 500 | Check `api-key` is valid for your provider |
