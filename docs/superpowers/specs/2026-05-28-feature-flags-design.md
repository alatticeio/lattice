---
title: Feature Flags 功能配置系统
---

# Feature Flags 功能配置系统

> Source: `internal/server/models/system_config.go`, `internal/server/controller/`, `internal/server/server/`, `frontend/src/stores/feature.ts`, `frontend/src/components/app-sidebar/AppSidebar.vue`

## Problem (现状)

前端管理界面的功能模块（AI Assistant、Agent Sandbox 等）全部硬编码展示，管理员无法控制功能的上线/下线：

1. **未开发完的功能对用户可见**：新功能即使未完成也会出现在侧边栏
2. **无法临时下线功能**：出现 bug 或需要维护时，只能改代码重新部署
3. **没有统一的功能开关机制**：唯一的动态开关是 landing page 的 `/api/v1/demo/status` 一次性检查，不可复用

## Design Decisions

| 决策项 | 选择 | 理由 |
|--------|------|------|
| 控制范围 | 前端侧边栏 + 路由守卫 | 简单直接，不做后端 API 强制校验 |
| 变更传播 | 下次刷新生效 | 不搞实时推送，降低复杂度 |
| 粒度 | 模块级（整个功能组/页面） | 一个 flag 控制一个功能模块 |
| Flag 管理 | 代码声明定义，DB 存状态 | 防止管理员随意创建，保证一致性 |
| 存储 | 复用 `SystemConfig` key-value 表 | 无需新建表，key 前缀 `feature.` |
| Admin 分组 | 不受 feature flag 控制 | Platform Admin 始终对 platform_admin 可见 |

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  Admin UI (settings/features)                       │
│  PUT /api/v1/features/:key { enabled: true/false }  │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  SystemConfig Table (la_system_config)               │
│  key: "feature.ai_assistant"  value: "true"          │
│  key: "feature.agent_sandbox" value: "false"         │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  GET /api/v1/features                                │
│  → 合并代码定义 + DB 状态 → 返回完整 flag 列表       │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  Frontend Feature Store (Pinia)                      │
│  登录后拉取，缓存在内存，刷新页面时更新              │
└──────────────────────┬───────────────────────────────┘
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
     Sidebar       Route Guard   FeatureGate
     过滤渲染      拦截访问       组件级控制
```

## Feature Flag 定义

代码中声明的 flag 列表（`models.FeatureFlagDefs`）：

| Key | Label | 控制范围 | 默认 |
|-----|-------|----------|------|
| `feature.ai_assistant` | AI Assistant | AI 整个 group（Chat/Intent/Debug/Compliance/Tools/MCP） | enabled |
| `feature.agent_sandbox` | Agent Sandbox | Sandbox 整个 group（List/Tokens/Audit） | enabled |
| `feature.alerts` | Alerts | 告警相关页面 | enabled |
| `feature.monitor` | Monitor | 监控页面 | enabled |

新增功能只需：
1. `models/system_config.go` 中加一行常量 + `FeatureFlagDefs` 条目
2. `AppSidebar.vue` 中对应 nav item 加 `featureKey` 字段

## API

### GET /api/v1/features

所有已登录用户可读。返回合并了代码定义和 DB 状态的完整 flag 列表。

```json
{
  "code": 200,
  "data": [
    { "key": "feature.ai_assistant", "label": "AI Assistant", "group": "ai", "enabled": true },
    { "key": "feature.agent_sandbox", "label": "Agent Sandbox", "group": "sandbox", "enabled": false }
  ]
}
```

DB 中无记录时使用代码定义的 `Default` 值。

### PUT /api/v1/features/:key

仅 platform_admin。切换指定 feature 的 enabled 状态。

```json
{ "enabled": false }
```

## Frontend Integration

### Store

`useFeatureStore` 提供：
- `flags: Record<string, boolean>` — key → enabled 映射
- `fetchFeatures()` — 登录后调用，从 API 加载
- `isEnabled(key: string): boolean` — 查询 flag，默认 true

### Sidebar Filtering

`AppSidebar.vue` 的 `navMain` computed 中：
- Group 级：`featureKey` disabled → 整个 group 不渲染
- Item 级：`featureKey` disabled → 从 items 中过滤掉
- Platform Admin group：不受 feature flag 控制

### Route Guard

`authGuard.ts` 中新增检查：
- `to.meta.featureKey` 存在且对应 feature disabled → 重定向 `/dashboard`

### Admin Page

`/settings/features` 页面（Platform Admin only）：
- 按 group 分组展示所有 flag
- Switch 开关切换，调 API 更新

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/server/models/system_config.go` | Modify | 添加 FeatureKey 常量和 FeatureFlagDef |
| `internal/server/dto/feature.go` | New | FeatureFlagItem, UpdateFeatureFlagRequest |
| `internal/server/controller/feature.go` | New | FeatureController 接口和实现 |
| `internal/server/server/feature.go` | New | 路由 handler |
| `internal/server/server/server.go` | Modify | 添加 featureController 字段 |
| `internal/server/server/api.go` | Modify | 注册 featureRouter |
| `frontend/src/api/feature.ts` | New | API service |
| `frontend/src/stores/feature.ts` | New | Feature store |
| `frontend/src/stores/user.ts` | Modify | fetchUserInfo 后加载 features |
| `frontend/src/router/guard/authGuard.ts` | Modify | 添加 featureKey 检查 |
| `frontend/src/types/route-meta.d.ts` | Modify | 添加 featureKey 字段 |
| `frontend/src/components/app-sidebar/AppSidebar.vue` | Modify | 侧边栏 feature 过滤 |
| `frontend/src/pages/settings/features/index.vue` | New | 管理页面 |
| `frontend/src/locales/en/common.json` | Modify | 添加 i18n key |
| `frontend/src/locales/zh-CN/common.json` | Modify | 添加 i18n key |
| `frontend/src/locales/en/settings.json` | Modify | 添加页面 i18n |
| `frontend/src/locales/zh-CN/settings.json` | Modify | 添加页面 i18n |
