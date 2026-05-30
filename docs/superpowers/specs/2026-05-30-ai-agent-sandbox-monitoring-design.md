# AI Agent 沙箱监控与安全审计设计

**日期**：2026-05-30  
**状态**：Draft  
**关联文档**：[AI Agent Zero Trust 安全网络设计](./2026-05-30-ai-agent-zero-trust-network-design.md)、[Agent Sandbox](./2026-05-29-agent-sandbox-design.md)

---

## 背景与动机

### 问题

当前 gVisor 集成仅提供 syscall 级别的隔离，无法监控 AI Agent 的应用层行为：

| 能力 | gVisor 能做 | gVisor 不能做 |
|------|------------|--------------|
| 系统调用过滤 | ✅ | - |
| 文件系统隔离 | ✅ | - |
| 网络隔离 | ✅ | - |
| **工具调用监控** | ❌ | 看不到应用层 |
| **SQL 注入检测** | ❌ | 看不到 SQL 内容 |
| **危险命令拦截** | ❌ | 只看到 exec() |
| **数据泄露检测** | ❌ | 看不到传输内容 |

### 安全审计的核心需求

企业需要知道 AI Agent 的**应用层行为**，而不仅仅是系统调用：

- Agent 调用了哪些工具（db:query、file:read、shell:exec）
- 工具调用的参数（SQL 查询、文件路径、命令内容）
- 工具调用的结果（成功/失败、返回数据）
- 异常行为检测（SQL 注入、路径遍历、危险命令）

### 解决方案：三层防御

```
┌─────────────────────────────────────────────────────────────┐
│                    第一层：工具调用拦截                       │
│   - MCP Proxy / Agent SDK Hook                              │
│   - 身份验证 + 权限检查 + 参数检查                           │
│   - 实时审计日志                                            │
│   → 阻止 90% 的恶意行为                                     │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    第二层：eBPF 策略执行                      │
│   - 网络策略（LatticePolicy）                               │
│   - 文件访问控制                                            │
│   - 进程白名单                                              │
│   → 阻止 Agent 逃逸后的横向移动                             │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    第三层：gVisor 沙箱（兜底）                │
│   - syscall 过滤                                            │
│   - 内核漏洞防护                                            │
│   → 阻止 0-day 漏洞利用                                     │
└─────────────────────────────────────────────────────────────┘
```

---

## 监控层次架构

```
┌─────────────────────────────────────────────────────────────┐
│                      AI Agent 进程                          │
│   ┌─────────────────────────────────────────────────────┐  │
│   │                Agent SDK / Runtime                   │  │
│   │   tool_call("db:query", {sql: "SELECT..."})        │  │
│   └─────────────────────────────────────────────────────┘  │
│                            │                                │
│                            ▼                                │
│   ┌─────────────────────────────────────────────────────┐  │
│   │              工具调用拦截层（Lattice 注入）            │  │
│   │                                                     │  │
│   │   1. 身份验证：AgentIdentity 是否有效？              │  │
│   │   2. 权限检查：工具在 allowedTools 白名单？          │  │
│   │   3. 参数检查：SQL 有注入？路径有遍历？              │  │
│   │   4. 风险评分：异常行为？频率过高？                  │  │
│   │   5. 审计日志：记录完整调用链                        │  │
│   │                                                     │  │
│   │   ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐             │  │
│   │   │身份 │  │权限 │  │参数 │  │审计 │             │  │
│   │   │验证 │→│检查 │→│检查 │→│日志 │             │  │
│   │   └─────┘  └─────┘  └─────┘  └─────┘             │  │
│   └─────────────────────────────────────────────────────┘  │
│                            │                                │
│                            ▼                                │
│   ┌─────────────────────────────────────────────────────┐  │
│   │                   工具执行层                         │  │
│   │   实际执行：db:query → 数据库                       │  │
│   └─────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   gVisor 沙箱（可选）                        │
│   - syscall 隔离（兜底）                                    │
│   - 文件系统隔离                                            │
│   - 网络隔离（netstack）                                    │
└─────────────────────────────────────────────────────────────┘
```

---

## 第一层：工具调用拦截

### 拦截方式 1：MCP Proxy（已有）

```
Agent → MCP Proxy → MCP Server
         ↓
    拦截 tool_call
    检查权限、参数
    记录审计日志
```

**优点**：已实现，透明拦截  
**缺点**：只对 MCP 协议有效

### 拦截方式 2：Agent SDK Hook

```go
// Agent 初始化时注入 hook
agent := lattice.NewAgent(lattice.AgentConfig{
    Identity: "payment-agent",
    ToolHook: func(ctx context.Context, tool string, params map[string]interface{}) error {
        // 1. 权限检查
        if !identity.AllowedTools.Contains(tool) {
            return fmt.Errorf("tool %s not allowed", tool)
        }
        
        // 2. 参数检查
        if err := checkParams(tool, params); err != nil {
            return err
        }
        
        // 3. 审计日志
        audit.Log(ctx, tool, params)
        
        return nil // 放行
    },
})
```

**优点**：通用，不依赖 MCP  
**缺点**：需要 Agent 使用 Lattice SDK

---

## 参数级安全检查

### SQL 注入检测

```go
func checkSQL(sql string) error {
    // 简单规则
    dangerous := []string{
        "DROP TABLE", "DELETE FROM", "UPDATE.*SET",
        "INSERT INTO", "ALTER TABLE", "EXEC(",
        "UNION SELECT", "--", ";",
    }
    
    upper := strings.ToUpper(sql)
    for _, pattern := range dangerous {
        if matched, _ := regexp.MatchString(pattern, upper); matched {
            return fmt.Errorf("potentially dangerous SQL: %s", pattern)
        }
    }
    
    return nil
}
```

### 路径遍历检测

```go
func checkPath(path string) error {
    // 检测路径遍历
    if strings.Contains(path, "..") {
        return fmt.Errorf("path traversal detected: %s", path)
    }
    
    // 检测敏感目录
    sensitive := []string{"/etc", "/var", "/root", "/home"}
    for _, dir := range sensitive {
        if strings.HasPrefix(path, dir) {
            return fmt.Errorf("access to sensitive directory: %s", dir)
        }
    }
    
    return nil
}
```

### 危险命令检测

```go
func checkCommand(cmd string) error {
    dangerous := []string{
        "rm -rf", "dd if=", "mkfs", "chmod 777",
        "curl.*|sh", "wget.*|sh", "eval(",
        "sudo", "su -",
    }
    
    for _, pattern := range dangerous {
        if matched, _ := regexp.MatchString(pattern, cmd); matched {
            return fmt.Errorf("dangerous command: %s", pattern)
        }
    }
    
    return nil
}
```

---

## 审计日志设计

### 日志格式

```json
{
  "timestamp": "2026-05-30T10:15:30Z",
  "agent_id": "payment-agent",
  "peer_id": "device-abc",
  "peer_identity": "production-db-server",
  "action": "tool_call",
  "tool": "db:query",
  "params": {
    "sql": "SELECT * FROM orders WHERE user_id = 123"
  },
  "result": "success",
  "rows_affected": 5,
  "sandbox": "gvisor",
  "policy_matched": "payment-to-db",
  "risk_score": 15,
  "checks": {
    "identity_valid": true,
    "tool_allowed": true,
    "params_safe": true
  }
}
```

### 审计能力

- **实时审计流**：WebSocket/SSE 推送
- **历史查询**：按 Agent、工具、时间范围
- **合规报告导出**：SOC2、ISO27001 模板
- **异常告警**：风险评分超阈值

---

## 第二层：eBPF 策略执行

### 网络策略（已有）

通过 `LatticePolicy` 控制 Agent 的网络访问：

```yaml
apiVersion: lattice.io/v1alpha1
kind: LatticePolicy
spec:
  network: production
  ingress:
  - from:
    - identityRef: payment-agent
    ports:
    - port: 5432
  egress:
  - to:
    - identityRef: database-service
    ports:
    - port: 5432
```

### 文件访问控制（新增）

通过 eBPF hook 限制 Agent 的文件访问：

```go
// eBPF 程序：限制文件访问
SEC("lsm/file_open")
int BPF_PROG(restrict_file_open, struct file *file) {
    // 获取当前进程的 AgentIdentity
    u32 agent_id = get_current_agent_id();
    
    // 检查文件路径是否在白名单
    char path[256];
    get_file_path(file, path, sizeof(path));
    
    if (!is_path_allowed(agent_id, path)) {
        return -EACCES; // 拒绝访问
    }
    
    return 0; // 允许访问
}
```

### 进程白名单（新增）

通过 eBPF hook 限制 Agent 可以执行的进程：

```go
// eBPF 程序：限制进程执行
SEC("lsm/bprm_check_security")
int BPF_PROG(restrict_process_exec, struct linux_binprm *bprm) {
    // 获取当前进程的 AgentIdentity
    u32 agent_id = get_current_agent_id();
    
    // 检查进程是否在白名单
    char comm[16];
    get_process_name(bprm, comm, sizeof(comm));
    
    if (!is_process_allowed(agent_id, comm)) {
        return -EACCES; // 拒绝执行
    }
    
    return 0; // 允许执行
}
```

---

## 第三层：gVisor 沙箱

### 自动安装

```go
func ensureGVisor() (string, error) {
    runscPath := filepath.Join(homeDir(), ".lattice", "bin", "runsc")
    
    // 检查是否已安装
    if _, err := os.Stat(runscPath); err == nil {
        return runscPath, nil
    }
    
    // 自动下载
    log.Info("Installing gVisor...")
    url := getGVisorDownloadURL(runtime.GOOS, runtime.GOARCH)
    if err := downloadAndExtract(url, runscPath); err != nil {
        return "", fmt.Errorf("install gVisor: %w", err)
    }
    
    return runscPath, nil
}
```

### 集成方式

```go
func runWithGVisor(args []string) error {
    // 确保 gVisor 已安装
    runscPath, err := ensureGVisor()
    if err != nil {
        return err
    }
    
    // 构造 runsc 命令
    cmd := exec.Command(runscPath, "run",
        "--network=none",  // 禁用 runsc 网络，用 Lattice 的
        "--file-access=exclusive",
        "--hostname=lattice-sandbox",
        sandboxID,
    )
    
    // 设置 Agent 进程
    cmd.Args = append(cmd.Args, args...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    return cmd.Run()
}
```

---

## 风险评分系统

### 评分规则

```go
type RiskScore struct {
    ToolRisk    int // 工具风险（0-100）
    ParamRisk   int // 参数风险（0-100）
    Frequency   int // 调用频率（0-100）
    History     int // 历史行为（0-100）
}

func calculateRiskScore(ctx context.Context, tool string, params map[string]interface{}) RiskScore {
    score := RiskScore{}
    
    // 1. 工具风险
    switch tool {
    case "shell:exec":
        score.ToolRisk = 80
    case "file:write":
        score.ToolRisk = 60
    case "db:query":
        score.ToolRisk = 40
    default:
        score.ToolRisk = 20
    }
    
    // 2. 参数风险
    if err := checkParams(tool, params); err != nil {
        score.ParamRisk = 90
    }
    
    // 3. 调用频率
    freq := getCallFrequency(ctx, tool)
    if freq > 100 { // 每分钟超过 100 次
        score.Frequency = 80
    }
    
    // 4. 历史行为
    history := getAgentHistory(ctx)
    if history.Violations > 0 {
        score.History = 70
    }
    
    return score
}
```

### 告警阈值

```go
func shouldAlert(score RiskScore) bool {
    // 总分超过 70 触发告警
    total := score.ToolRisk + score.ParamRisk + score.Frequency + score.History
    return total > 280 // 平均 70
}
```

---

## 实现范围

### Phase 1（1 个月）：MCP Proxy 增强

1. 参数检查库（SQL、路径、命令）
2. 审计日志增强（记录完整调用链）
3. 风险评分基础版

### Phase 2（2 个月）：Agent SDK Hook

1. 通用工具拦截 API
2. 身份验证 + 权限检查
3. 实时审计流

### Phase 3（4 个月）：gVisor 集成

1. 自动安装 gVisor
2. 三层防御完整实现
3. eBPF 策略执行（文件、进程）

---

## 配置示例

### Agent 配置

```yaml
apiVersion: lattice.io/v1alpha1
kind: AgentIdentity
metadata:
  name: payment-agent
spec:
  peerRef: agent-pod-abc123
  allowedTools:
    - "db:query"
    - "api:call"
    - "file:read"
  allowedNamespaces:
    - "production"
  sandbox: gvisor
  auditLevel: full
  enforcementMode: enforce
  expiresAt: "2026-06-01T00:00:00Z"
  riskThreshold: 70
```

### 全局配置

```yaml
# Lattice 控制面配置
sandbox:
  gvisor:
    autoInstall: true
    version: "latest"
  audit:
    enabled: true
    level: "full"
    storage: "local" # 或 "s3"
  riskScoring:
    enabled: true
    threshold: 70
    alertWebhook: "https://hooks.slack.com/xxx"
```

---

## 竞争优势

| 能力 | Lattice | Tailscale | Istio | 传统 VPN |
|------|---------|-----------|-------|----------|
| 工具调用监控 | ✅ | ❌ | ❌ | ❌ |
| SQL 注入检测 | ✅ | ❌ | ❌ | ❌ |
| 路径遍历检测 | ✅ | ❌ | ❌ | ❌ |
| 危险命令拦截 | ✅ | ❌ | ❌ | ❌ |
| 风险评分 | ✅ | ❌ | ❌ | ❌ |
| 实时审计 | ✅ | 基础 | 基础 | ❌ |
| gVisor 隔离 | ✅ | ❌ | ❌ | ❌ |
| eBPF 策略 | ✅ | ❌ | ❌ | ❌ |
