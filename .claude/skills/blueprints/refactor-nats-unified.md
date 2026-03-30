# Blueprint: NATS Unified Communication

## 重构目标
取消 Agent 对 Manager HTTP API 的依赖，实现单连接（Single Connection）通信。

## 技术规范
1. **通信协议**: 使用 NATS Core Request-Response 处理同步请求（如获取配置）。
2. **主题规范**:
    - `wf.req.{agent_id}.config`: 请求配置
    - `wf.event.{agent_id}.status`: 上报状态
3. **错误处理**:
    - 必须处理 NATS `ErrNoResponders`（表示 Manager 掉线）。
    - 实现请求超时与重试逻辑。
4. **安全**: 使用 NATS Nkey 或 JWT 进行身份标识。