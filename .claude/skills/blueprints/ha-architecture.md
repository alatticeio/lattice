# Blueprint: Enterprise HA & Decoupled Architecture

## Architecture Rules
1. **NATS Decoupling**:
    - 生产环境禁用 `nats.RunEmbedded`。
    - 支持通过 `WF_NATS_URL` 连接外部高可用 NATS JetStream 集群。
2. **Database Hardening**:
    - 禁用 SQLite，强制使用 MySQL 8.0+ 或 PostgreSQL。
    - 数据库连接需配置 `MaxOpenConns` 和 `ConnMaxLifetime` 以适配高并发 Agent 请求。
3. **Security**:
    - NATS 通讯必须强制开启 TLS。
    - Manager API 必须对接企业 OIDC (如 Okta, Keycloak) 或 LDAP。

## Cluster Specifics
- **Offline Registry**: 必须支持通过私有镜像仓库（如 Harbor）分发。
- **Topology Awareness**: Controller 需感知算力节点所在的机架位，优化隧道拓扑。