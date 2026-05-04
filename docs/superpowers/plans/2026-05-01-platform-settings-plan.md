# Platform Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a platform-level settings page where admins can configure NATS URL via UI, stored in DB and served via discovery endpoint.

**Architecture:** Key-value `SystemConfig` model in DB → GORM repository → thin controller → server handler (no service layer — get/set is too simple to warrant indirection). Frontend: new settings page under `/settings/platform` with single NATS URL form.

**Tech Stack:** Go/GORM, Gin, Vue 3 + shadcn/vue + Tailwind

---

## File Structure

### Backend — Create
| File | Responsibility |
|------|---------------|
| `internal/server/models/system_config.go` | `SystemConfig` model + key constants |
| `internal/db/gormstore/system_config.go` | GORM implementation of `SystemConfigRepository` |
| `internal/server/dto/platform.go` | Request/response DTOs |
| `internal/server/controller/platform.go` | Thin controller wrapping store |

### Backend — Modify
| File | Change |
|------|--------|
| `internal/agent/store/store.go` | Add `SystemConfigRepository` interface + `SystemConfig()` to `Store` |
| `internal/db/gormstore/store.go` | Register `SystemConfigRepository` field + accessor |
| `internal/db/gormstore/migrate.go` | Add `&models.SystemConfig{}` to AutoMigrate |
| `internal/server/server/api.go` | Add discovery DB lookup + `s.platformRouter()` call |
| `internal/server/server/server.go` | Add `PlatformSettings` config key field + platformRouter declaration |

### Frontend — Create
| File | Responsibility |
|------|---------------|
| `fronted/src/api/platform.ts` | API call functions |
| `fronted/src/pages/settings/platform/index.vue` | Platform settings page |

### Frontend — Modify
| File | Change |
|------|--------|
| `fronted/src/components/app-sidebar/AppSidebar.vue` | Add "Platform Settings" nav item to Platform Admin group |
| `fronted/src/locales/en/settings.json` | Add platform section i18n keys |
| `fronted/src/locales/zh-CN/settings.json` | Add platform section i18n keys |
| `fronted/src/locales/en/common.json` | Add `nav.platformSettings` key |
| `fronted/src/locales/zh-CN/common.json` | Add `nav.platformSettings` key |

---

### Task 1: SystemConfig Model + GORM Repository

**Files:**
- Create: `internal/server/models/system_config.go`
- Create: `internal/db/gormstore/system_config.go`
- Modify: `internal/agent/store/store.go`
- Modify: `internal/db/gormstore/store.go`
- Modify: `internal/db/gormstore/migrate.go`

- [ ] **Step 1: Create SystemConfig model**

Write `internal/server/models/system_config.go`:

```go
package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	ConfigKeyNatsURL = "nats_url"
)

type SystemConfig struct {
	Key       string         `gorm:"primaryKey;type:varchar(128);not null" json:"key"`
	Value     string         `gorm:"type:text" json:"value"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (SystemConfig) TableName() string { return "la_system_config" }
```

- [ ] **Step 2: Add SystemConfigRepository to Store interface**

In `internal/agent/store/store.go`, add to the existing `Store` interface:

```go
// SystemConfigRepository manages platform-level key-value configuration.
type SystemConfigRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) (map[string]string, error)
}
```

Add to `Store` interface:

```go
SystemConfig() SystemConfigRepository
```

- [ ] **Step 3: Create GORM implementation**

Write `internal/db/gormstore/system_config.go`:

```go
package gormstore

import (
	"context"

	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
)

type systemConfigRepo struct {
	db *gorm.DB
}

func newSystemConfigRepo(db *gorm.DB) *systemConfigRepo {
	return &systemConfigRepo{db: db}
}

func (r *systemConfigRepo) Get(ctx context.Context, key string) (string, error) {
	var cfg models.SystemConfig
	err := r.db.WithContext(ctx).Take(&cfg, "`key` = ?", key).Error
	if err != nil {
		return "", err
	}
	return cfg.Value, nil
}

func (r *systemConfigRepo) Set(ctx context.Context, key, value string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&models.SystemConfig{}).Where("`key` = ?", key).Count(&count)
		if count == 0 {
			return tx.Create(&models.SystemConfig{Key: key, Value: value}).Error
		}
		return tx.Model(&models.SystemConfig{}).Where("`key` = ?", key).Update("value", value).Error
	})
}

func (r *systemConfigRepo) GetAll(ctx context.Context) (map[string]string, error) {
	var rows []models.SystemConfig
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		result[row.Key] = row.Value
	}
	return result, nil
}
```

- [ ] **Step 4: Register SystemConfigRepository in GormStore**

In `internal/db/gormstore/store.go`:
- Add `systemConfig store.SystemConfigRepository` to `GormStore` struct
- Add `return s.systemConfig` accessor method
- Add `systemConfig: newSystemConfigRepo(db),` to `newStore()`

Edit `GormStore` struct:

```go
type GormStore struct {
	db                   *gorm.DB
	users                store.UserRepository
	workspaces           store.WorkspaceRepository
	workspaceMembers     store.WorkspaceMemberRepository
	profiles             store.ProfileRepository
	userIdentities       store.UserIdentityRepository
	workspaceInvitations store.WorkspaceInvitationRepository
	auditLogs            store.AuditLogRepository
	workflowRequests     store.WorkflowRepository
	policies             store.PolicyRepository
	alerts               store.AlertRepository
	customMetrics        store.CustomMetricRepository
	systemConfig         store.SystemConfigRepository
}
```

Add accessor:

```go
func (s *GormStore) SystemConfig() store.SystemConfigRepository { return s.systemConfig }
```

Add to `newStore()`:

```go
systemConfig: newSystemConfigRepo(db),
```

- [ ] **Step 5: Register AutoMigrate**

In `internal/db/gormstore/migrate.go`, add `&models.SystemConfig{}` to the AutoMigrate call:

```go
func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.UserProfile{},
		&models.UserIdentity{},
		&models.Workspace{},
		&models.WorkspaceMember{},
		&models.WorkspaceInvitation{},
		&models.AuditLog{},
		&models.WorkflowRequest{},
		&models.Policy{},
		&models.AlertRule{},
		&models.AlertHistory{},
		&models.AlertChannel{},
		&models.AlertSilence{},
		&models.CustomMetric{},
		&models.SystemConfig{},
	)
}
```

- [ ] **Step 6: Verify compilation**

Run: `make build SERVICE=latticed`
Expected: Build succeeds

- [ ] **Step 7: Commit**

```bash
git add internal/server/models/system_config.go internal/db/gormstore/system_config.go internal/agent/store/store.go internal/db/gormstore/store.go internal/db/gormstore/migrate.go
git commit -m "feat: add SystemConfig model and repository for platform settings"
```

---

### Task 2: Platform Settings API

**Files:**
- Create: `internal/server/dto/platform.go`
- Create: `internal/server/controller/platform.go`
- Create: `internal/server/server/platform.go`
- Modify: `internal/server/server/api.go`
- Modify: `internal/server/server/server.go`

- [ ] **Step 1: Create DTO**

Write `internal/server/dto/platform.go`:

```go
package dto

type PlatformSettingsRequest struct {
	NatsURL string `json:"nats_url"`
}

type PlatformSettingsResponse struct {
	NatsURL string `json:"nats_url"`
}
```

- [ ] **Step 2: Create PlatformController**

Write `internal/server/controller/platform.go`:

```go
package controller

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
)

type PlatformController interface {
	GetSettings(ctx context.Context) (*dto.PlatformSettingsResponse, error)
	UpdateSettings(ctx context.Context, req dto.PlatformSettingsRequest) error
}

type platformController struct {
	store store.Store
}

func NewPlatformController(st store.Store) PlatformController {
	return &platformController{store: st}
}

func (c *platformController) GetSettings(ctx context.Context) (*dto.PlatformSettingsResponse, error) {
	val, err := c.store.SystemConfig().Get(ctx, models.ConfigKeyNatsURL)
	if err != nil {
		// Not found → empty string is fine
		return &dto.PlatformSettingsResponse{}, nil
	}
	return &dto.PlatformSettingsResponse{NatsURL: val}, nil
}

func (c *platformController) UpdateSettings(ctx context.Context, req dto.PlatformSettingsRequest) error {
	return c.store.SystemConfig().Set(ctx, models.ConfigKeyNatsURL, req.NatsURL)
}
```

- [ ] **Step 3: Create API routes**

Write `internal/server/server/platform.go`:

```go
package server

import (
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) platformRouter() {
	r := s.Group("/api/v1/settings/platform")
	r.Use(middleware.AuthMiddleware(s.revocationList))
	// Only platform_admin can read/write platform settings
	r.Use(s.middleware.PlatformAdminOnly())
	{
		r.GET("", s.getPlatformSettings())
		r.PUT("", s.updatePlatformSettings())
	}
}

func (s *Server) getPlatformSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := s.platformController.GetSettings(c.Request.Context())
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, settings)
	}
}

func (s *Server) updatePlatformSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.PlatformSettingsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request body")
			return
		}
		if err := s.platformController.UpdateSettings(c.Request.Context(), req); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
```

Note: `PlatformAdminOnly()` already exists in `internal/server/server/middleware/permission.go:136`.

- [ ] **Step 4: Register platformRouter in server**

Add `platformController` field to `Server` struct in `internal/server/server/server.go`:

```go
platformController controller.PlatformController
```

Initialize it in `NewServer` alongside other controllers:

```go
platformController: controller.NewPlatformController(st),
```

Add `s.platformRouter()` call in `apiRouter()` before the SPA handler in `internal/server/server/api.go`:

```go
s.platformRouter()
// Discovery — no auth required; returns NATS URL for agent auto-connect.
api.GET("/discovery", s.handleDiscovery())
```

- [ ] **Step 5: Update discovery to read from DB**

Modify `handleDiscovery()` in `internal/server/server/api.go`:

```go
func (s *Server) handleDiscovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		natsURL := s.cfg.SignalingURL
		// Prefer DB-stored value (set via platform settings UI)
		if dbURL, err := s.store.SystemConfig().Get(c.Request.Context(), models.ConfigKeyNatsURL); err == nil && dbURL != "" {
			natsURL = dbURL
		}
		if natsURL == "" {
			natsURL = "nats://127.0.0.1:4222"
		}
		resp.OK(c, gin.H{"nats_url": natsURL})
	}
}
```

Add `"github.com/alatticeio/lattice/internal/server/models"` to the imports in `api.go`.

- [ ] **Step 6: Verify compilation**

Run: `make build SERVICE=latticed`
Expected: Build succeeds

- [ ] **Step 7: Commit**

```bash
git add internal/server/dto/platform.go internal/server/controller/platform.go internal/server/server/platform.go internal/server/server/api.go internal/server/server/server.go
git commit -m "feat: add platform settings API with NATS URL configuration"
```

---

### Task 3: Frontend — API Layer + Nav Entry + i18n

**Files:**
- Create: `fronted/src/api/platform.ts`
- Modify: `fronted/src/components/app-sidebar/AppSidebar.vue`
- Modify: `fronted/src/locales/en/common.json`
- Modify: `fronted/src/locales/zh-CN/common.json`
- Modify: `fronted/src/locales/en/settings.json`
- Modify: `fronted/src/locales/zh-CN/settings.json`

- [ ] **Step 1: Create API module**

Write `fronted/src/api/platform.ts`:

```typescript
import request from '@/api/request'

export interface PlatformSettings {
  nats_url: string
}

export const getPlatformSettings = () =>
  request.get<PlatformSettings>('/settings/platform')

export const updatePlatformSettings = (data: PlatformSettings) =>
  request.put('/settings/platform', data)
```

- [ ] **Step 2: Add i18n keys — common.json (en)**

In `fronted/src/locales/en/common.json`, add to `nav` object:

```json
"platformSettings": "Platform Settings"
```

- [ ] **Step 3: Add i18n keys — common.json (zh-CN)**

In `fronted/src/locales/zh-CN/common.json`, add to `nav` object:

```json
"platformSettings": "平台设置"
```

- [ ] **Step 4: Add i18n keys — settings.json (en)**

In `fronted/src/locales/en/settings.json`, add at the end (before closing `}`):

```json
"platform": {
  "title": "Platform Settings",
  "desc": "Manage platform-level configuration for all workspaces and agents.",
  "natsUrlLabel": "NATS Signaling URL",
  "natsUrlPlaceholder": "nats://nats-cluster:4222",
  "natsUrlHint": "Agent nodes use this URL to connect to the signaling server. Format: nats://host:port",
  "saveBtn": "Save Changes",
  "saving": "Saving...",
  "saved": "Platform settings saved",
  "saveFailed": "Failed to save platform settings",
  "loadFailed": "Failed to load platform settings",
  "validationRequired": "NATS URL is required",
  "validationPrefix": "NATS URL must start with nats:// or nats+tls://"
}
```

- [ ] **Step 5: Add i18n keys — settings.json (zh-CN)**

In `fronted/src/locales/zh-CN/settings.json`, add at the end:

```json
"platform": {
  "title": "平台设置",
  "desc": "管理全局平台配置，影响所有工作空间和 Agent 节点。",
  "natsUrlLabel": "NATS 信令地址",
  "natsUrlPlaceholder": "nats://nats-cluster:4222",
  "natsUrlHint": "Agent 节点通过此地址连接信令服务器。格式：nats://host:port",
  "saveBtn": "保存更改",
  "saving": "保存中...",
  "saved": "平台设置已保存",
  "saveFailed": "保存失败",
  "loadFailed": "加载失败",
  "validationRequired": "NATS URL 不能为空",
  "validationPrefix": "NATS URL 必须以 nats:// 或 nats+tls:// 开头"
}
```

- [ ] **Step 6: Add nav entry in sidebar**

In `fronted/src/components/app-sidebar/AppSidebar.vue`, add Platform Settings to the Platform Admin section:

Find the `platform` group items array and add:

```typescript
{ title: t('common.nav.platformSettings'), url: "/settings/platform" },
```

It should look like:

```typescript
...(isAdmin ? [{
  title: t('common.nav.group.platform'),
  url: "#",
  icon: ShieldCheck,
  items: [
    { title: t('common.nav.platformSettings'), url: "/settings/platform" },
    { title: t('common.nav.users'),          url: "/manage/users" },
    { title: t('common.nav.workspaces'),     url: "/manage/workspaces" },
    { title: t('common.nav.networkPeering'), url: "/platform/network-peering" },
    { title: t('common.nav.clusterPeering'), url: "/platform/cluster-peering" },
    { title: t('common.nav.approvals'),      url: "/settings/approvals" },
  ],
}] : []),
```

- [ ] **Step 7: Commit**

```bash
git add fronted/src/api/platform.ts fronted/src/components/app-sidebar/AppSidebar.vue fronted/src/locales/en/common.json fronted/src/locales/zh-CN/common.json fronted/src/locales/en/settings.json fronted/src/locales/zh-CN/settings.json
git commit -m "feat: add platform settings API, nav entry, and i18n keys"
```

---

### Task 4: Frontend — Platform Settings Page

**Files:**
- Create: `fronted/src/pages/settings/platform/index.vue`

- [ ] **Step 1: Create the platform settings page**

Write `fronted/src/pages/settings/platform/index.vue`:

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'
import { Save, Loader2, Server } from 'lucide-vue-next'
import {
  getPlatformSettings,
  updatePlatformSettings,
} from '@/api/platform'

definePage({
  meta: { titleKey: 'settings.platform.title', descKey: 'settings.platform.desc' },
})

const { t } = useI18n()

const natsUrl = ref('')
const loading = ref(false)
const saving = ref(false)
const loaded = ref(false)

async function fetchSettings() {
  loading.value = true
  try {
    const { data } = await getPlatformSettings() as any
    natsUrl.value = data?.nats_url ?? ''
  } catch {
    toast.error(t('settings.platform.loadFailed'))
  } finally {
    loading.value = false
    loaded.value = true
  }
}

function validate(): string | null {
  const v = natsUrl.value.trim()
  if (!v) return t('settings.platform.validationRequired')
  if (!v.startsWith('nats://') && !v.startsWith('nats+tls://')) {
    return t('settings.platform.validationPrefix')
  }
  return null
}

async function handleSave() {
  const err = validate()
  if (err) {
    toast.error(err)
    return
  }
  saving.value = true
  try {
    await updatePlatformSettings({ nats_url: natsUrl.value.trim() })
    toast.success(t('settings.platform.saved'))
  } catch {
    toast.error(t('settings.platform.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(fetchSettings)
</script>

<template>
  <div class="flex flex-col gap-6 p-6 animate-in fade-in duration-300">
    <!-- Header -->
    <div>
      <h1 class="text-xl font-semibold tracking-tight">{{ t('settings.platform.title') }}</h1>
      <p class="text-sm text-muted-foreground mt-1">{{ t('settings.platform.desc') }}</p>
    </div>

    <!-- Loading state -->
    <div v-if="loading" class="flex items-center justify-center py-16">
      <Loader2 class="size-6 animate-spin text-muted-foreground" />
    </div>

    <!-- Settings form -->
    <div v-else class="max-w-xl space-y-6">
      <!-- NATS URL -->
      <div class="space-y-2">
        <label class="text-sm font-medium flex items-center gap-1.5">
          <Server class="size-4 text-muted-foreground" />
          {{ t('settings.platform.natsUrlLabel') }}
        </label>
        <Input
          v-model="natsUrl"
          :placeholder="t('settings.platform.natsUrlPlaceholder')"
          class="font-mono text-sm"
        />
        <p class="text-xs text-muted-foreground">{{ t('settings.platform.natsUrlHint') }}</p>
      </div>

      <!-- Save button -->
      <Button :disabled="saving" @click="handleSave" class="gap-1.5">
        <Save v-if="!saving" class="size-4" />
        <Loader2 v-else class="size-4 animate-spin" />
        {{ saving ? t('settings.platform.saving') : t('settings.platform.saveBtn') }}
      </Button>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd fronted && pnpm build`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add fronted/src/pages/settings/platform/index.vue
git commit -m "feat: add platform settings page with NATS URL configuration form"
```

---

### Verification

- [ ] **Step 1: Full build check**

Run: `make build`
Expected: All services build without errors

- [ ] **Step 2: Frontend build check**

Run: `cd fronted && pnpm build`
Expected: Build succeeds

- [ ] **Step 3: Final commit for any remaining changes**

```bash
git status  # verify clean working tree
```
