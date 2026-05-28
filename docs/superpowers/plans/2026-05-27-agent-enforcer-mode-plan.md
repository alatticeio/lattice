# Agent Enforcer Mode Selection — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give users and platform admins control over the agent policy enforcement backend (iptables vs eBPF) via a four-layer priority: CLI flag > user personal setting > server global default > auto-detect.

**Architecture:** Add `EnforcerMode string` field to Config, pass it to `SelectEnforcerMode()` which returns the validated backend. Server-side: discovery API returns global default, NATS register response carries user-specific preference from their settings. Agent-side: apply values in priority order after discovery + registration.

**Tech Stack:** Go 1.25, cobra CLI, Gin HTTP, NATS signaling, mapstructure config

---

### Task 1: Config — Add `EnforcerMode` field

**Files:**
- Modify: `internal/agent/config/config.go:283-284` (add field near `WgPort`)
- Modify: `internal/agent/config/config.go` (add default in `setDefaults()`)

- [ ] **Step 1: Add field to Config struct**

在 `WgPort` 之后添加 `EnforcerMode` 字段：

```go
// internal/agent/config/config.go:284-285 area

WgPort        int    `mapstructure:"wg-port"` // WireGuard/ICE UDP listen port, default 51820
EnforcerMode  string `mapstructure:"enforcer-mode"` // "auto", "iptables", "ebpf"
```

- [ ] **Step 2: Set default in `setDefaults()`**

找到 `setDefaults()` 函数（约第 599 行），在 WgPort 默认设置之后添加：

```go
// in setDefaults()
v.SetDefault("enforcer-mode", "auto")
```

- [ ] **Step 3: Verify compilation**

```bash
cd /Users/francis/workspc/lattice && go build ./internal/agent/config/...
```
Expected: build success

- [ ] **Step 4: Commit**

```bash
git add internal/agent/config/config.go
git commit -s -m "feat(agent): add EnforcerMode field to agent Config"
```

---

### Task 2: Selector — Update `SelectEnforcerMode` to accept Config

**Files:**
- Modify: `internal/agent/provision/enforcer_selector.go:44-52`

- [ ] **Step 1: Rewrite `SelectEnforcerMode` signature and logic**

```go
// internal/agent/provision/enforcer_selector.go

import (
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
)

// SelectEnforcerMode decides which PolicyEnforcer backend to use.
// cfg.EnforcerMode may be "auto", "iptables", or "ebpf".
// "auto" defers to build-tag detection (community → iptables, pro → kernel probe).
// "ebpf" falls back to iptables with a warning if eBPF is unavailable.
func SelectEnforcerMode(cfg *config.Config, logger *log.Logger) EnforcerMode {
	switch cfg.EnforcerMode {
	case "iptables":
		logger.Info("policy enforcement backend: iptables (source: explicit)")
		return ModeIPTables
	case "ebpf":
		if mode := selectEBPFAvailable(); mode == ModeEBPF {
			logger.Info("policy enforcement backend: eBPF (source: explicit)")
			return ModeEBPF
		}
		logger.Warn("ebpf requested but unavailable, falling back to iptables")
		return ModeIPTables
	default: // "auto" or empty
		mode := selectEBPFAvailable()
		if mode == ModeEBPF {
			logger.Info("policy enforcement backend: eBPF (source: auto)")
			return ModeEBPF
		}
		logger.Info("policy enforcement backend: iptables (source: auto)")
		return ModeIPTables
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/francis/workspc/lattice && go build ./internal/agent/provision/...
```
Expected: build success

- [ ] **Step 3: Commit**

```bash
git add internal/agent/provision/enforcer_selector.go
git commit -s -m "feat(agent): update SelectEnforcerMode to accept Config with explicit mode"
```

---

### Task 3: Agent Discovery — Add `EnforcerMode` to `discoveryResult`

**Files:**
- Modify: `internal/agent/node.go:62-66` (discoveryResult struct)
- Modify: `internal/agent/node.go:69-96` (discover function)
- Modify: `internal/agent/node.go:251-264` (applying discovery results)

- [ ] **Step 1: Add `EnforcerMode` to `discoveryResult`**

```go
// internal/agent/node.go — discoveryResult struct
type discoveryResult struct {
	NatsURL      string
	StunURL      string
	EnforcerMode string // server global default for enforcer mode
}
```

- [ ] **Step 2: Parse `enforcer_mode` in `discover()`**

```go
// internal/agent/node.go — inside discover() function
var envelope struct {
	Data struct {
		NatsURL      string `json:"nats_url"`
		StunURL      string `json:"stun_url"`
		EnforcerMode string `json:"enforcer_mode"` // new
	} `json:"data"`
}
// ... after json.Decode ...

return discoveryResult{
	NatsURL:      envelope.Data.NatsURL,
	StunURL:      envelope.Data.StunURL,
	EnforcerMode: envelope.Data.EnforcerMode,
}, nil
```

- [ ] **Step 3: Apply discovery `EnforcerMode` in NewNode**

在 discovery 代码块（第 251-264 行区域），STUN URL 赋值之后添加：

```go
// Apply server global enforcer_mode default if CLI hasn't overridden it.
if config.Conf.EnforcerMode == "" || config.Conf.EnforcerMode == "auto" {
	if d.EnforcerMode != "" {
		config.Conf.EnforcerMode = d.EnforcerMode
		log.GetLogger("node").Info("Discovered enforcer mode", "mode", d.EnforcerMode)
	}
}
```

- [ ] **Step 4: Verify compilation**

```bash
cd /Users/francis/workspc/lattice && go build ./internal/agent/...
```
Expected: build success

- [ ] **Step 5: Commit**

```bash
git add internal/agent/node.go
git commit -s -m "feat(agent): extract enforcer_mode from discovery and apply as fallback"
```

---

### Task 4: Agent Register — Add `EnforcerMode` to `infra.Peer`

**Files:**
- Modify: `internal/agent/infra/message.go:120-147` (Peer struct)

- [ ] **Step 1: Add `EnforcerMode` to Peer struct**

```go
// internal/agent/infra/message.go — Peer struct, before Labels
type Peer struct {
	// ...existing fields...
	Token       string            `json:"token,omitempty"`
	LrpUrl      string            `json:"lrpUrl,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	EnforcerMode string           `json:"enforcerMode,omitempty"` // user personal setting from registration
}
```

- [ ] **Step 2: Apply user enforcer_mode from registration in NewNode**

在 `internal/agent/node.go` 中，注册完成之后（`node.current` 赋值之后），添加：

```go
// Apply user personal enforcer_mode from registration response.
// Overrides discovery default.
if node.current.EnforcerMode != "" {
	config.Conf.EnforcerMode = node.current.EnforcerMode
	log.GetLogger("node").Info("Enforcer mode from user setting", "mode", node.current.EnforcerMode)
}
// Ensure non-empty before selector runs.
if config.Conf.EnforcerMode == "" {
	config.Conf.EnforcerMode = "auto"
}
```

这段代码插入位置：`node.current = ...` 赋值之后、`privateKey, err = utils.ParseKey(node.current.PrivateKey)` 之前。

- [ ] **Step 3: Update `SelectEnforcerMode` call to pass Config**

在 `internal/agent/node.go` 第 414 行：

```go
// before:
enforcerMode := provision.SelectEnforcerMode(cfg.Logger)
// after:
enforcerMode := provision.SelectEnforcerMode(cfg.Flags, cfg.Logger)
```

- [ ] **Step 4: Verify compilation**

```bash
cd /Users/francis/workspc/lattice && go build ./internal/agent/...
```
Expected: build success

- [ ] **Step 5: Check `sandbox_register.go` and `sandbox_agent.go` references to `infra.Peer` don't break**

```bash
cd /Users/francis/workspc/lattice && go build ./cmd/lattice/cmd/sandbox/...
```
Expected: build success

- [ ] **Step 6: Commit**

```bash
git add internal/agent/infra/message.go internal/agent/node.go
git commit -s -m "feat(agent): apply user enforcer_mode from NATS registration response"
```

---

### Task 5: CLI — Add `--enforcer-mode` flag to `lattice up`

**Files:**
- Modify: `cmd/lattice/cmd/up.go:109-119` (flag registration area)

- [ ] **Step 1: Add flag**

在 `upCmd()` 的 flag 注册区，`--wg-port` 之后添加：

```go
fs.IntP("wg-port", "", 51820, "UDP port for WireGuard and ICE (default 51820)")
fs.StringP("enforcer-mode", "", "auto", "policy enforcement backend: auto, iptables, ebpf")
fs.StringP("name", "", "", "display name for this node (shown in the UI)")
```

- [ ] **Step 2: Marshal flag value to config**

在 `upCmd()` 的 `RunE` 里，现有的 `save` 处理之后、`agent.Start` 之前添加：

```go
// Propagate --enforcer-mode CLI flag to config.
if mode, _ := cmd.Flags().GetString("enforcer-mode"); mode != "" {
	config.Conf.EnforcerMode = mode
}
```

放在现有 `if name, _ := cmd.Flags().GetString("name"); name != "" {` 代码块附近。

- [ ] **Step 3: Verify compilation**

```bash
cd /Users/francis/workspc/lattice && go build ./cmd/lattice/...
```
Expected: build success

- [ ] **Step 4: Commit**

```bash
git add cmd/lattice/cmd/up.go
git commit -s -m "feat(cli): add --enforcer-mode flag to lattice up"
```

---

### Task 6: Server Discovery — Return `enforcer_mode` in API

**Files:**
- Modify: `internal/server/server/api.go:327-348` (handleDiscovery function)

- [ ] **Step 1: Add `enforcer_mode` to discovery response**

```go
// internal/server/server/api.go — handleDiscovery()
func (s *Server) handleDiscovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ...existing NATS URL lookup...

		// ...existing STUN URL lookup...

		// Enforcer mode: server global default from config.
		enforcerMode := s.cfg.EnforcerMode
		if enforcerMode == "" {
			enforcerMode = "auto"
		}

		resp.OK(c, gin.H{
			"nats_url":      natsURL,
			"stun_url":      stunURL,
			"enforcer_mode": enforcerMode,
		})
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/francis/workspc/lattice && go build ./internal/server/server/...
```
Expected: build success

- [ ] **Step 3: Commit**

```bash
git add internal/server/server/api.go
git commit -s -m "feat(server): return enforcer_mode in discovery API response"
```

---

### Task 7: User Settings — Add `EnforcerMode` to profile API

**Files:**
- Modify: `internal/server/dto/user.go:4-14` (UserSettingsResponse)
- Modify: `internal/server/dto/user.go:17-30` (UpdateSettingsRequest)
- Modify: `internal/server/models/user.go:28-36` (UserProfile)
- Modify: `internal/server/service/user_profile.go` (ProfileService — to read/write)

- [ ] **Step 1: Add field to `UserSettingsResponse` DTO**

```go
// internal/server/dto/user.go
type UserSettingsResponse struct {
	// ...existing fields...
	Language     string `json:"language"`
	EmailNotify  bool   `json:"emailNotify"`
	EnforcerMode string `json:"enforcerMode"` // new: "auto", "iptables", "ebpf"
}
```

- [ ] **Step 2: Add field to `UpdateSettingsRequest` DTO**

```go
type UpdateSettingsRequest struct {
	// ...existing fields...
	Language     string `json:"language"`
	EmailNotify  bool   `json:"emailNotify"`
	EnforcerMode string `json:"enforcerMode"` // new
}
```

- [ ] **Step 3: Add field to `UserProfile` model**

```go
// internal/server/models/user.go
type UserProfile struct {
	UserID       string `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	Title        string `gorm:"size:128" json:"title"`
	Company      string `gorm:"size:128" json:"company"`
	Bio          string `gorm:"type:text" json:"bio"`
	Timezone     string `gorm:"size:64;default:'Asia/Shanghai'" json:"timezone"`
	Language     string `gorm:"size:16;default:'zh-CN'" json:"language"`
	EmailNotify  bool   `gorm:"default:true" json:"emailNotify"`
	EnforcerMode string `gorm:"size:16;default:'auto'" json:"enforcerMode"` // new
}
```

- [ ] **Step 4: Update `ProfileService` to read/write `EnforcerMode`**

```go
// internal/server/service/user_profile.go — GetProfile
response := &dto.UserSettingsResponse{
	// ...existing mappings...
	Language:     profile.Language,
	EmailNotify:  profile.EmailNotify,
	EnforcerMode: profile.EnforcerMode, // new
}

// internal/server/service/user_profile.go — UpdateProfile, in Upsert
return tx.Profiles().Upsert(ctx, &models.UserProfile{
	UserID:       userID,
	// ...existing fields...
	Language:     req.Language,
	EmailNotify:  req.EmailNotify,
	EnforcerMode: req.EnforcerMode, // new
})
```

- [ ] **Step 5: Verify compilation**

```bash
cd /Users/francis/workspc/lattice && go build ./internal/server/...
```
Expected: build success

- [ ] **Step 6: Commit**

```bash
git add internal/server/dto/user.go internal/server/models/user.go internal/server/service/user_profile.go
git commit -s -m "feat(server): add EnforcerMode to user profile settings"
```

---

### Task 8: Server Registration — Return user enforcer_mode in NATS register response

**Files:**
- Modify: `internal/server/server/server.go:480-501` (handleSandboxNATSRegister)
- Need to look up: token → workspace → workspace owner → user settings → enforcer_mode
- Check how `agentRegistrationService.RegisterAgent` can pass back enforcer_mode

- [ ] **Step 1: Find the token's workspace owner and read their user settings**

`RegisterAgent` (in `agent_registration.go`) already has access to the enrollment token, which includes `AllowedNamespace` (workspace namespace). From there:

1. Query workspace by namespace → get workspace owner (userId)
2. Query user settings by userId → get EnforcerMode

This lookup happens inside `RegisterAgent`. Modify `AgentRegisterResponse` to carry the value:

```go
// internal/server/service/agent_registration.go
type AgentRegisterResponse struct {
	JWT               string `json:"jwt"`
	AgentIdentityName string `json:"agentIdentityName"`
	EnforcerMode      string `json:"enforcerMode,omitempty"` // new
}
```

- [ ] **Step 2: Look up user's enforcer_mode in `RegisterAgent`**

In `RegisterAgent()`, after validating the token and before returning. The token has `tok.AllowedNamespace`. Look up the workspace by namespace to find its owner (`CreatedBy`), then read that user's profile for `EnforcerMode`:

```go
// After token validation, lookup user enforcer preference
var userEnforcerMode string
workspace, wsErr := s.store.Workspaces().GetByNamespace(ctx, tok.AllowedNamespace)
if wsErr == nil && workspace.CreatedBy != "" {
	profile, profErr := s.store.Profiles().Get(ctx, workspace.CreatedBy)
	if profErr == nil && profile.EnforcerMode != "" {
		userEnforcerMode = profile.EnforcerMode
	}
}
```

- [ ] **Step 3: Pass `EnforcerMode` back in the response**

```go
return &AgentRegisterResponse{
	JWT:               jwtStr,
	AgentIdentityName: req.AgentName,
	EnforcerMode:      userEnforcerMode,
}, nil
```

- [ ] **Step 4: Pass `EnforcerMode` through `handleSandboxNATSRegister` to `infra.Peer`**

```go
// internal/server/server/server.go — handleSandboxNATSRegister
returnPeer := &infra.Peer{
	Name:          peer.AppID,
	AppID:         peer.AppID,
	Token:         result.JWT,
	EnforcerMode:  result.EnforcerMode, // new
}
```

- [ ] **Step 5: Handle regular (non-sandbox) NATS registration**

For the `s.peerController.Register(ctx, content)` path (regular agent, line 474), the `PeerService.Register()` returns `*infra.Peer`. Modify it to also set `EnforcerMode` from user profile after token validation.

```go
// internal/server/service/peer.go — Register(), after token validation
workspace, wsErr := p.store.Workspaces().GetByNamespace(ctx, token.Namespace)
if wsErr == nil && workspace.CreatedBy != "" {
	profile, profErr := p.store.Profiles().Get(ctx, workspace.CreatedBy)
	if profErr == nil && profile.EnforcerMode != "" {
		node.EnforcerMode = profile.EnforcerMode
	}
}
```

- [ ] **Step 6: Verify compilation**

```bash
cd /Users/francis/workspc/lattice && go build ./internal/server/...
```
Expected: build success

- [ ] **Step 7: Commit**

```bash
git add internal/server/service/agent_registration.go internal/server/service/peer.go internal/server/server/server.go
git commit -s -m "feat(server): pass user enforcer_mode through NATS registration response"
```

---

### Task 9: Helm — Add `enforcerMode` to chart

**Files:**
- Modify: `deploy/charts/lattice/values.yaml`
- Modify: `deploy/charts/lattice/templates/configmap.yaml`

- [ ] **Step 1: Add `enforcerMode` to values.yaml**

在 `deploy/charts/lattice/values.yaml` 的 `config` 块，`signalingUrl` 附近添加：

```yaml
  # -- Global default enforcer mode returned by /api/v1/discovery.
  # Agents use this when neither CLI flag nor user setting is configured.
  # Valid values: auto, iptables, ebpf
  enforcerMode: ""
```

- [ ] **Step 2: Add `enforcer-mode` to configmap template**

在 `deploy/charts/lattice/templates/configmap.yaml`，`stun-url` 块之后添加：

```yaml
    {{- if .Values.config.enforcerMode }}
    enforcer-mode: "{{ .Values.config.enforcerMode }}"
    {{- end }}
```

- [ ] **Step 3: Verify with helm lint**

```bash
cd /Users/francis/workspc/lattice && helm lint deploy/charts/lattice/
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add deploy/charts/lattice/values.yaml deploy/charts/lattice/templates/configmap.yaml
git commit -s -m "feat(helm): add enforcerMode to server config"
```

---

### Task 10: Lint and Final Verification

- [ ] **Step 1: Run full lint**

```bash
cd /Users/francis/workspc/lattice && make lint
```
Expected: 0 issues

- [ ] **Step 2: Run unit tests**

```bash
cd /Users/francis/workspc/lattice && make test
```
Expected: all tests pass

- [ ] **Step 3: Verify full build**

```bash
cd /Users/francis/workspc/lattice && make build
cd /Users/francis/workspc/lattice && make EDITION=pro build
```
Expected: both builds succeed
