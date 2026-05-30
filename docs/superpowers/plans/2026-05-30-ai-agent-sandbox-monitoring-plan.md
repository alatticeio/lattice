# AI Agent 沙箱监控实现计划

**日期**：2026-05-30  
**Spec**：[2026-05-30-ai-agent-sandbox-monitoring-design.md](../specs/2026-05-30-ai-agent-sandbox-monitoring-design.md)  
**状态**：Ready

---

## 总体时间估算

| 阶段 | 工作量 | 内容 |
|------|--------|------|
| 参数检查库 | 1 天 | SQL、路径、命令检查 |
| MCP Proxy 增强 | 2 天 | 拦截 + 审计日志 |
| Agent SDK Hook | 2 天 | 通用工具拦截 API |
| 风险评分系统 | 1 天 | 风险计算 + 告警 |
| gVisor 集成 | 2 天 | 自动安装 + 运行时集成 |
| 测试 | 2 天 | 单元测试 + E2E |
| **合计** | **10 天** | |

---

## 第一阶段：参数检查库（1 天）

### 步骤 1.1：创建安全检查包

**文件**：`internal/agent/security/checker.go`

```go
package security

// Checker 提供工具调用参数的安全检查
type Checker interface {
    CheckSQL(sql string) error
    CheckPath(path string) error
    CheckCommand(cmd string) error
    Check(tool string, params map[string]interface{}) error
}

type checker struct{}

func NewChecker() Checker {
    return &checker{}
}
```

### 步骤 1.2：SQL 注入检测

**文件**：`internal/agent/security/sql.go`

```go
func (c *checker) CheckSQL(sql string) error {
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

### 步骤 1.3：路径遍历检测

**文件**：`internal/agent/security/path.go`

```go
func (c *checker) CheckPath(path string) error {
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

### 步骤 1.4：危险命令检测

**文件**：`internal/agent/security/command.go`

```go
func (c *checker) CheckCommand(cmd string) error {
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

### 步骤 1.5：统一检查接口

**文件**：`internal/agent/security/checker.go`

```go
func (c *checker) Check(tool string, params map[string]interface{}) error {
    switch {
    case strings.HasPrefix(tool, "db:"):
        if sql, ok := params["sql"].(string); ok {
            return c.CheckSQL(sql)
        }
    case strings.HasPrefix(tool, "file:"):
        if path, ok := params["path"].(string); ok {
            return c.CheckPath(path)
        }
    case strings.HasPrefix(tool, "shell:"):
        if cmd, ok := params["command"].(string); ok {
            return c.CheckCommand(cmd)
        }
    }
    
    return nil
}
```

### 步骤 1.6：单元测试

**文件**：`internal/agent/security/checker_test.go`

---

## 第二阶段：MCP Proxy 增强（2 天）

### 步骤 2.1：审计日志结构

**文件**：`internal/agent/audit/logger.go`

```go
package audit

type AuditEntry struct {
    Timestamp    time.Time              `json:"timestamp"`
    AgentID      string                 `json:"agent_id"`
    PeerID       string                 `json:"peer_id"`
    PeerIdentity string                 `json:"peer_identity"`
    Action       string                 `json:"action"`
    Tool         string                 `json:"tool"`
    Params       map[string]interface{} `json:"params"`
    Result       string                 `json:"result"`
    Sandbox      string                 `json:"sandbox"`
    PolicyMatch  string                 `json:"policy_matched"`
    RiskScore    int                    `json:"risk_score"`
    Checks       CheckResult            `json:"checks"`
}

type CheckResult struct {
    IdentityValid bool `json:"identity_valid"`
    ToolAllowed   bool `json:"tool_allowed"`
    ParamsSafe    bool `json:"params_safe"`
}

type Logger interface {
    Log(entry AuditEntry)
    LogError(entry AuditEntry, err error)
}
```

### 步骤 2.2：文件审计日志

**文件**：`internal/agent/audit/file_logger.go`

```go
type FileLogger struct {
    path string
    mu   sync.Mutex
}

func NewFileLogger(path string) *FileLogger {
    return &FileLogger{path: path}
}

func (l *FileLogger) Log(entry AuditEntry) {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return
    }
    defer f.Close()
    
    json.NewEncoder(f).Encode(entry)
}
```

### 步骤 2.3：MCP Proxy 拦截

**文件**：`internal/mcp/proxy.go`

在现有 MCP Proxy 中添加安全检查：

```go
func (p *Proxy) handleToolCall(ctx context.Context, req *ToolCallRequest) (*ToolCallResponse, error) {
    // 1. 获取 AgentIdentity
    identity := getIdentityFromContext(ctx)
    
    // 2. 权限检查
    if !identity.AllowedTools.Contains(req.Tool) {
        audit.Log(AuditEntry{
            Action: "tool_call",
            Tool:   req.Tool,
            Result: "denied",
            Checks: CheckResult{ToolAllowed: false},
        })
        return nil, fmt.Errorf("tool %s not allowed", req.Tool)
    }
    
    // 3. 参数检查
    checker := security.NewChecker()
    if err := checker.Check(req.Tool, req.Params); err != nil {
        audit.Log(AuditEntry{
            Action: "tool_call",
            Tool:   req.Tool,
            Result: "denied",
            Checks: CheckResult{ParamsSafe: false},
        })
        return nil, err
    }
    
    // 4. 执行工具调用
    resp, err := p.executeToolCall(ctx, req)
    
    // 5. 记录审计日志
    audit.Log(AuditEntry{
        Action: "tool_call",
        Tool:   req.Tool,
        Params: req.Params,
        Result: "success",
        Checks: CheckResult{
            IdentityValid: true,
            ToolAllowed:   true,
            ParamsSafe:    true,
        },
    })
    
    return resp, err
}
```

### 步骤 2.4：单元测试

**文件**：`internal/mcp/proxy_test.go`

---

## 第三阶段：Agent SDK Hook（2 天）

### 步骤 3.1：Hook 接口定义

**文件**：`pkg/agent/hook.go`

```go
package agent

// ToolHook 是工具调用前的拦截函数
type ToolHook func(ctx context.Context, tool string, params map[string]interface{}) error

// HookConfig 配置 Agent 的拦截行为
type HookConfig struct {
    ToolHook    ToolHook
    AuditLogger audit.Logger
}
```

### 步骤 3.2：Agent 配置扩展

**文件**：`pkg/agent/config.go`

```go
type Config struct {
    Identity    string
    ServerURL   string
    Token       string
    Hooks       HookConfig
    Sandbox     string
    AuditLevel  string
}
```

### 步骤 3.3：工具调用包装

**文件**：`pkg/agent/agent.go`

```go
func (a *Agent) CallTool(ctx context.Context, tool string, params map[string]interface{}) (interface{}, error) {
    // 1. 执行 Hook
    if a.hooks.ToolHook != nil {
        if err := a.hooks.ToolHook(ctx, tool, params); err != nil {
            return nil, err
        }
    }
    
    // 2. 执行实际工具调用
    result, err := a.executeTool(ctx, tool, params)
    
    // 3. 记录审计日志
    if a.hooks.AuditLogger != nil {
        a.hooks.AuditLogger.Log(audit.AuditEntry{
            Action: "tool_call",
            Tool:   tool,
            Params: params,
            Result: "success",
        })
    }
    
    return result, err
}
```

### 步骤 3.4：默认 Hook 实现

**文件**：`pkg/agent/default_hook.go`

```go
func DefaultHook(identity *AgentIdentity) ToolHook {
    checker := security.NewChecker()
    
    return func(ctx context.Context, tool string, params map[string]interface{}) error {
        // 1. 权限检查
        if !identity.AllowedTools.Contains(tool) {
            return fmt.Errorf("tool %s not allowed", tool)
        }
        
        // 2. 参数检查
        return checker.Check(tool, params)
    }
}
```

### 步骤 3.5：单元测试

**文件**：`pkg/agent/agent_test.go`

---

## 第四阶段：风险评分系统（1 天）

### 步骤 4.1：风险评分结构

**文件**：`internal/agent/security/risk.go`

```go
package security

type RiskScore struct {
    ToolRisk    int `json:"tool_risk"`
    ParamRisk   int `json:"param_risk"`
    Frequency   int `json:"frequency"`
    History     int `json:"history"`
}

func (r RiskScore) Total() int {
    return r.ToolRisk + r.ParamRisk + r.Frequency + r.History
}

func (r RiskScore) Average() int {
    return r.Total() / 4
}
```

### 步骤 4.2：风险计算器

**文件**：`internal/agent/security/risk_calculator.go`

```go
type RiskCalculator struct {
    frequencyMap map[string]int
    historyMap   map[string]int
    mu           sync.Mutex
}

func NewRiskCalculator() *RiskCalculator {
    return &RiskCalculator{
        frequencyMap: make(map[string]int),
        historyMap:   make(map[string]int),
    }
}

func (rc *RiskCalculator) Calculate(ctx context.Context, tool string, params map[string]interface{}) RiskScore {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    
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
    checker := NewChecker()
    if err := checker.Check(tool, params); err != nil {
        score.ParamRisk = 90
    }
    
    // 3. 调用频率
    agentID := getAgentIDFromContext(ctx)
    rc.frequencyMap[agentID]++
    if rc.frequencyMap[agentID] > 100 {
        score.Frequency = 80
    }
    
    // 4. 历史行为
    if rc.historyMap[agentID] > 0 {
        score.History = 70
    }
    
    return score
}
```

### 步骤 4.3：告警触发

**文件**：`internal/agent/security/alert.go`

```go
type AlertManager struct {
    threshold int
    webhook   string
}

func NewAlertManager(threshold int, webhook string) *AlertManager {
    return &AlertManager{
        threshold: threshold,
        webhook:   webhook,
    }
}

func (am *AlertManager) Check(score RiskScore, entry audit.AuditEntry) {
    if score.Average() > am.threshold {
        am.sendAlert(entry)
    }
}

func (am *AlertManager) sendAlert(entry audit.AuditEntry) {
    // 发送到 webhook
    go func() {
        payload, _ := json.Marshal(entry)
        http.Post(am.webhook, "application/json", bytes.NewReader(payload))
    }()
}
```

### 步骤 4.4：单元测试

**文件**：`internal/agent/security/risk_test.go`

---

## 第五阶段：gVisor 集成（2 天）

### 步骤 5.1：gVisor 安装器

**文件**：`internal/agent/sandbox/gvisor.go`

```go
package sandbox

type GVisorInstaller struct {
    binDir string
}

func NewGVisorInstaller() *GVisorInstaller {
    home, _ := os.UserHomeDir()
    return &GVisorInstaller{
        binDir: filepath.Join(home, ".lattice", "bin"),
    }
}

func (gi *GVisorInstaller) EnsureInstalled() (string, error) {
    runscPath := filepath.Join(gi.binDir, "runsc")
    
    // 检查是否已安装
    if _, err := os.Stat(runscPath); err == nil {
        return runscPath, nil
    }
    
    // 自动下载
    log.Info("Installing gVisor...")
    url := gi.getDownloadURL()
    if err := gi.download(url, runscPath); err != nil {
        return "", fmt.Errorf("install gVisor: %w", err)
    }
    
    // 设置可执行权限
    os.Chmod(runscPath, 0755)
    
    return runscPath, nil
}

func (gi *GVisorInstaller) getDownloadURL() string {
    goos := runtime.GOOS
    goarch := runtime.GOARCH
    
    // gVisor 下载 URL 格式
    return fmt.Sprintf(
        "https://storage.googleapis.com/gvisor/releases/release/latest/%s/%s/runsc",
        goos, goarch,
    )
}

func (gi *GVisorInstaller) download(url, dest string) error {
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    os.MkdirAll(filepath.Dir(dest), 0755)
    
    out, err := os.Create(dest)
    if err != nil {
        return err
    }
    defer out.Close()
    
    _, err = io.Copy(out, resp.Body)
    return err
}
```

### 步骤 5.2：gVisor 运行器

**文件**：`internal/agent/sandbox/runner.go`

```go
type Runner struct {
    installer *GVisorInstaller
}

func NewRunner() *Runner {
    return &Runner{
        installer: NewGVisorInstaller(),
    }
}

func (r *Runner) RunWithGVisor(args []string) error {
    // 确保 gVisor 已安装
    runscPath, err := r.installer.EnsureInstalled()
    if err != nil {
        return err
    }
    
    // 构造 runsc 命令
    cmd := exec.Command(runscPath, "run",
        "--network=none",
        "--file-access=exclusive",
        "--hostname=lattice-sandbox",
        "lattice-sandbox",
    )
    
    // 设置 Agent 进程
    cmd.Args = append(cmd.Args, args...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    return cmd.Run()
}

func (r *Runner) RunWithoutSandbox(args []string) error {
    cmd := exec.Command(args[0], args[1:]...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    return cmd.Run()
}
```

### 步骤 5.3：CLI 集成

**文件**：`cmd/lattice/cmd/sandbox/run.go`

```go
func runSandbox(cmd *cobra.Command, args []string) error {
    isolation, _ := cmd.Flags().GetString("isolation")
    
    runner := sandbox.NewRunner()
    
    switch isolation {
    case "gvisor":
        return runner.RunWithGVisor(args)
    case "none":
        return runner.RunWithoutSandbox(args)
    default:
        // 自动检测
        if _, err := exec.LookPath("runsc"); err == nil {
            return runner.RunWithGVisor(args)
        }
        log.Warn("gVisor not found, running without sandbox")
        return runner.RunWithoutSandbox(args)
    }
}
```

### 步骤 5.4：单元测试

**文件**：`internal/agent/sandbox/gvisor_test.go`

---

## 第六阶段：测试（2 天）

### 步骤 6.1：单元测试

- `internal/agent/security/checker_test.go`
- `internal/agent/security/risk_test.go`
- `internal/agent/audit/logger_test.go`
- `internal/agent/sandbox/gvisor_test.go`

### 步骤 6.2：集成测试

**文件**：`test/integration/sandbox_monitoring_test.go`

```
场景 1：SQL 注入检测
- 调用 db:query with "SELECT * FROM users; DROP TABLE users"
- 验证被拦截

场景 2：路径遍历检测
- 调用 file:read with "../../../etc/passwd"
- 验证被拦截

场景 3：危险命令检测
- 调用 shell:exec with "rm -rf /"
- 验证被拦截

场景 4：审计日志记录
- 执行正常工具调用
- 验证审计日志包含完整信息

场景 5：风险评分
- 执行高风险操作
- 验证风险评分正确计算
```

### 步骤 6.3：E2E 测试

**文件**：`test/e2e/sandbox_monitoring_test.go`

```
场景 1：完整工具调用流程
- Agent 启动
- 调用工具
- 验证拦截、审计、风险评分

场景 2：gVisor 隔离
- 使用 --isolation=gvisor 启动
- 验证进程隔离

场景 3：MCP Proxy 拦截
- 通过 MCP Proxy 调用工具
- 验证拦截和审计
```

---

## 依赖关系

```
参数检查库 (1.1-1.5)
    ↓
MCP Proxy 增强 (2.1-2.3) ──→ 测试 (6.1-6.3)
    ↓
Agent SDK Hook (3.1-3.4)
    ↓
风险评分系统 (4.1-4.3)
    ↓
gVisor 集成 (5.1-5.3)
```

---

## 验收标准

1. **参数检查库**
   - SQL 注入检测准确率 > 95%
   - 路径遍历检测准确率 > 99%
   - 危险命令检测准确率 > 90%

2. **MCP Proxy 增强**
   - 所有工具调用被拦截和记录
   - 审计日志格式正确
   - 性能开销 < 5ms

3. **Agent SDK Hook**
   - Hook API 易用
   - 默认 Hook 实现完整
   - 文档清晰

4. **风险评分系统**
   - 风险计算准确
   - 告警触发及时
   - webhook 集成正常

5. **gVisor 集成**
   - 自动安装成功率 > 95%
   - 运行时隔离正常
   - 回退机制可靠
