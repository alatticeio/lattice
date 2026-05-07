# E2E 双模式测试设计

**日期：** 2026-05-02
**状态：** 待实现

## 背景

当前 e2e 测试代码存在多个 bug，且只支持单一场景（K8s 内 lattice-aio）。需要同时支持：

- **本地模式**：`latticed` 跑在宿主机（`localhost:8080`），agent 以 K8s Pod 形式跑在 k3d 集群
- **K8s 模式**：`latticed`（lattice-aio）部署在 K8s 集群中，agent 以 K8s Pod 形式跑在同一集群

## 现有 Bug 清单

| 位置 | 问题 | 影响 |
|------|------|------|
| `e2e_test.go:89` | `ns :=` shadowing，package 级 `ns` 从未更新 | ACL 测试在空 namespace 执行，必然失败 |
| `e2e_test.go:38` | 本地 `httpClient` 遮盖 suite 级变量 | suite 级 httpClient 配置完全失效 |
| `e2e_test.go:169` | `--signaling-url` 未在 `lattice up` 注册 | 容器启动即报 unknown flag |
| `e2e_test.go:108-115` | 查询 `lattice-nats-service` ClusterIP + `hostAliases` | 本地模式无此 Service，直接崩溃 |
| `e2e_test.go:168` | `--server-url` 硬编码 K8s 内部 DNS | 本地模式 agent pod 无法访问 |

## 架构设计

### 模式检测

```
--manage-url 为空  →  K8s 模式（自动程序化 port-forward）
--manage-url 非空  →  本地模式（直连指定地址）
```

### 网络拓扑

```
K8s 模式：
  测试进程 → port-forward → localhost:{随机端口} → lattice-api-service:8080
  agent pod → lattice-api-service.lattice-system.svc.cluster.local:8080
  agent pod → NATS (via discoverNATSURL，返回集群内 DNS)

本地模式：
  测试进程 → http://localhost:8080（直连）
  agent pod → http://host.docker.internal:8080（Docker Desktop 宿主机别名）
  agent pod → nats://host.docker.internal:4222（由 agent-nats-url 配置返回）
```

### Package 级变量

```go
var (
    restConfig      *rest.Config
    clientset       *kubernetes.Clientset
    latticeClient   client.Client
    httpClient      *http.Client      // 两种模式均为普通 HTTP client
    agentServerURL  string            // agent pod 使用的 server URL
    stopPortForward chan struct{}      // K8s 模式 port-forward 停止信号（nil = 本地模式）
    ns              string
    agentImage      string
    manageUrl       string
    kubeconfig      string
)
```

## 改动一：`e2e_suite_test.go`

### Flags

```
--agent-image      agent 镜像（默认 ghcr.io/winstonfly/lattice:e2e）
--manage-url       本地模式 API 地址（空 = K8s 模式）
--agent-server-url agent pod 访问 API 的地址（可选，空时自动推导）
--kubeconfig       kubeconfig 路径
```

### BeforeSuite 流程

```
1. 加载 kubeconfig，初始化 restConfig / clientset / latticeClient

2. if manageUrl != "":
     # 本地模式
     httpClient = &http.Client{Timeout: 30s}
     if agentServerURL == "":
       agentServerURL = replace(manageUrl, localhost/127.0.0.1 → host.docker.internal)
     调 settings API 写入 agent-nats-url（登录 → PATCH /api/v1/platform/settings）
   else:
     # K8s 模式
     找到 lattice-api-service 的 backing Pod
     程序化 port-forward pod:8080 → localhost:{随机端口}
     manageUrl = "http://localhost:{随机端口}"
     httpClient = &http.Client{Timeout: 30s}
     if agentServerURL == "":
       agentServerURL = "http://lattice-api-service.lattice-system.svc.cluster.local:8080"

3. 验证连通性：GET {manageUrl}/api/v1/discovery，超时 30s
```

### AfterSuite 流程

```
if stopPortForward != nil:
  close(stopPortForward)  // 关闭 port-forward goroutine
清理测试 Namespace（测试通过时）
```

### 程序化 Port-Forward 实现要点

使用 `k8s.io/client-go/tools/portforward`：

1. 用 label selector 找到 `lattice-api-service` 的某个 Running Pod
2. 用 `net.Listen("tcp", ":0")` 获取随机空闲端口
3. 用 `portforward.New(dialer, ports, stopCh, readyCh, stdout, stderr)` 启动
4. 等待 `readyCh` 确认就绪
5. 返回本地端口和 `stopCh`

## 改动二：Server Discovery 接口

### 新增 model key

```go
// internal/server/models/system_config.go
ConfigKeyAgentNatsURL = "agent-nats-url"
```

### `handleDiscovery()` 优先级

```
1. DB 中 agent-nats-url（专供 agent pod 外部访问）
2. DB 中 nats-url（原有逻辑）
3. config.SignalingURL
4. 兜底 "nats://127.0.0.1:4222"
```

### Platform Settings API

`PATCH /api/v1/platform/settings` 支持写入 `agent_nats_url` 字段，DTO 同步扩展。

### 使用场景

| 模式 | 配置方式 | discovery 返回 |
|------|----------|----------------|
| K8s 模式 | 无需配置 | `nats://lattice-nats-service.lattice-system.svc.cluster.local:4222` |
| 本地模式 | BeforeSuite 调 settings API 写入 | `nats://host.docker.internal:4222` |

## 改动三：`e2e_test.go`

1. **删除** 本地 `httpClient` 变量声明（第 38 行）
2. **修复** `ns :=` → `ns =`（第 89 行）
3. **删除** `--signaling-url` 参数（第 169 行）
4. **删除** `hostAliases` 和 `lattice-nats-service` ClusterIP 查询（第 108-115 行）
5. **替换** `--server-url` 硬编码值 → `agentServerURL` 变量

## 改动四：Makefile

```makefile
# K8s 模式
test-e2e:
    go test ./test/e2e/... -v -timeout 15m -args \
        --agent-image=$(LOCAL_AGENT_IMAGE) \
        --kubeconfig=$(E2E_KUBECONFIG)

# 本地模式（需先手动启动 latticed）
test-e2e-local:
    @test -n "$(MANAGE_URL)" || (echo "❌ 请设置 MANAGE_URL" && exit 1)
    go test ./test/e2e/... -v -timeout 15m -args \
        --agent-image=$(LOCAL_AGENT_IMAGE) \
        --kubeconfig=$(E2E_KUBECONFIG) \
        --manage-url=$(MANAGE_URL) \
        $(if $(AGENT_SERVER_URL),--agent-server-url=$(AGENT_SERVER_URL),)
```

## 运行方式

```bash
# K8s 模式
make e2e

# 本地模式（macOS Docker Desktop）
./bin/latticed &
make test-e2e-local MANAGE_URL=http://localhost:8080

# 本地模式（Linux 原生 k3d，需指定宿主机 IP）
make test-e2e-local MANAGE_URL=http://localhost:8080 AGENT_SERVER_URL=http://172.17.0.1:8080
```

## 不在本次范围内

- `lattice up` CLI 增加 `--signaling-url` flag（选择了 server discovery 方案）
- agent 在本地模式下以本地进程运行
- E2E 测试的 CI/CD 自动化流程调整
