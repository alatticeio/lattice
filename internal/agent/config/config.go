// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package config implements a unified configuration center using an "onion model" load priority:
//
//	Defaults < lattice.yaml < lattice.{env}.yaml < Environment variables (LATTICE_*) < CLI arguments
//	                                               < K8s service discovery fallback (only for empty values)
//
// Recommended usage (in cmd's PersistentPreRunE):
//
//	if err := config.GetManager().Load(cmd); err != nil { return err }
//
// After loading, access via config.GlobalConfig or config.Conf (both point to the same object).
// At service entry points that require validation of critical connection addresses, additionally call config.ValidateAndReport(config.GlobalConfig, isServer).
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	wflog "github.com/alatticeio/lattice/internal/agent/log"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var log = wflog.GetLogger("config")

// ─────────────────────────────────────────────
//
//	Global singleton
//
// ─────────────────────────────────────────────

var (
	managerOnce   sync.Once
	globalManager *ConfigManager

	// GlobalConfig / Conf point to the same Config object, serving as the single source of truth after loading.
	// Server-side components should use GlobalConfig; existing flat-field callers may use Conf directly.
	GlobalConfig = &Config{}
	Conf         = GlobalConfig
)

// GetManager returns the global singleton ConfigManager (thread-safe).
func GetManager() *ConfigManager {
	managerOnce.Do(func() {
		globalManager = &ConfigManager{v: viper.New()}
	})
	return globalManager
}

// NewConfigManager is a backward-compatible alias for GetManager.
func NewConfigManager() *ConfigManager { return GetManager() }

// ─────────────────────────────────────────────
//
//	ConfigManager
//
// ─────────────────────────────────────────────

// ConfigManager wraps a Viper instance and provides multi-environment loading capability.
type ConfigManager struct {
	v    *viper.Viper
	once sync.Once
	dir  string // resolved config directory (determined by --config-dir / LATTICE_CONFIG_DIR / default)
}

// Viper exposes the underlying instance for callers that need fine-grained control.
func (cm *ConfigManager) Viper() *viper.Viper { return cm.v }

// Load loads configuration using the "onion model", executing only once (idempotent).
//
//  1. Hard-coded defaults
//  2. lattice.yaml (base config)
//  3. lattice.{env}.yaml (environment-specific config, MergeInConfig)
//  4. Environment variables (LATTICE_ prefix)
//  5. CLI arguments (BindPFlags, highest priority)
//  6. K8s service discovery fallback (only for fields that remain empty)
//  7. Database driver inference
//
// --save: merges explicitly specified CLI arguments into the config file (without overwriting other existing entries).
func (cm *ConfigManager) Load(cmd *cobra.Command) error {
	var err error
	cm.once.Do(func() { err = cm.load(cmd) })
	return err
}

// LoadConf is an alias for Load, kept for backward compatibility.
func (cm *ConfigManager) LoadConf(cmd *cobra.Command) error { return cm.Load(cmd) }

// Save writes the current in-memory configuration (including merged results from all layers) back to the config file.
func (cm *ConfigManager) Save() error {
	path := cm.dir + "/lattice.yaml"
	if err := cm.v.WriteConfig(); err != nil {
		return cm.v.WriteConfigAs(path)
	}
	return nil
}

// SaveChangedFlags only writes flags explicitly changed on the command line to the config file,
// without overwriting other existing entries or writing default values that were not explicitly set by the user.
//
// Compare with Save(): Save() writes all known Viper keys (including defaults);
// SaveChangedFlags() only writes keys actually used in this run's --xxx arguments.
func (cm *ConfigManager) SaveChangedFlags(cmd *cobra.Command) error {
	path := cm.dir + "/lattice.yaml"
	if err := os.MkdirAll(cm.dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Inject explicitly specified CLI flags into Viper (skip --save itself)
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if f.Name == "save" {
			return
		}
		cm.v.Set(f.Name, cm.v.Get(f.Name))
	})

	// WriteConfig writes Viper's currently Set keys (including entries read from the config file),
	// without writing defaults that were only set via SetDefault and not overridden by any layer.
	if err := cm.v.WriteConfig(); err != nil {
		return cm.v.WriteConfigAs(path)
	}
	return nil
}

func (cm *ConfigManager) load(cmd *cobra.Command) error {
	v := cm.v
	v.SetConfigType("yaml")

	// ── Layer 1: defaults ──────────────────────────────────────────
	setDefaults(v)

	// ── Determine config directory (peek early) ──────────────────
	cm.dir = peekConfigDir(cmd)

	// ── Determine runtime environment (peek early, does not require full load) ──
	env := peekEnv(cmd)

	// ── Layer 2: lattice.yaml ─────────────────────────────────────
	baseFile := cm.dir + "/lattice.yaml"
	v.SetConfigFile(baseFile)
	if _, err := os.Stat(baseFile); os.IsNotExist(err) {
		log.Info("config file not found, writing defaults", "path", baseFile)
		if err2 := os.MkdirAll(cm.dir, 0o755); err2 != nil {
			log.Warn("failed to create config dir", "err", err2)
		}
		if err2 := v.SafeWriteConfigAs(baseFile); err2 != nil {
			log.Warn("failed to write default config file", "err", err2)
		}
	}
	if err := v.ReadInConfig(); err != nil {
		log.Warn("failed to read config file, ignoring", "err", err)
	}

	// ── Layer 3: lattice.{env}.yaml ────────────────────────────────
	envFile := fmt.Sprintf("%s/lattice.%s.yaml", cm.dir, env)
	v.SetConfigFile(envFile)
	if _, err := os.Stat(envFile); err == nil {
		if err := v.MergeInConfig(); err != nil {
			log.Warn("failed to merge env config", "file", envFile, "err", err)
		} else {
			log.Info("env config loaded", "file", envFile)
		}
	}
	// Reset to baseFile so subsequent WriteConfig / Save writes to the correct path
	v.SetConfigFile(baseFile)

	// ── Layer 4: environment variables (LATTICE_APP_LISTEN, LATTICE_SIGNALING_URL, ...)
	v.SetEnvPrefix("LATTICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// ── Layer 5: CLI arguments (BindPFlags auto-binds all flags) ────
	if err := v.BindPFlags(cmd.Flags()); err != nil {
		return fmt.Errorf("[config] BindPFlags failed: %w", err)
	}

	// ── Unmarshal: GlobalConfig and Conf share the same pointer ────
	if err := v.Unmarshal(GlobalConfig); err != nil {
		return fmt.Errorf("[config] Unmarshal failed: %w", err)
	}
	Conf = GlobalConfig

	// ── Compatibility: old vm-endpoint key (kebab-case → vmEndpoint) ──
	if GlobalConfig.Telemetry.VMEndpoint == "" {
		if oldEp := v.GetString("vm-endpoint"); oldEp != "" {
			GlobalConfig.Telemetry.VMEndpoint = oldEp
		}
	}

	// ── Layer 6: K8s service discovery fallback (only for fields that remain empty) ──
	applyK8sFallbacks(GlobalConfig)

	// ── Database driver inference (auto-detect driver from DSN format) ────
	inferDatabaseDriver(GlobalConfig)

	log.Debug("config loaded", "env", env, "listen", GlobalConfig.Listen, "driver", GlobalConfig.Database.Driver)

	// ── --save: persist CLI flags explicitly specified this run ────
	// Typical usage: lattice up --server-url http://y --save
	if f := cmd.Flags().Lookup("save"); f != nil && f.Value.String() == "true" {
		if err := cm.SaveChangedFlags(cmd); err != nil {
			log.Warn("failed to save config", "err", err)
		} else {
			log.Info("config saved", "path", baseFile)
		}
	}
	return nil
}

// ─────────────────────────────────────────────
//
//	Config: the single configuration struct
//
// ─────────────────────────────────────────────

// Config is the unified configuration struct for the entire project.
//
// Top-level flat fields correspond to CLI flag names (directly mapped by BindPFlags);
// Nested sub-structs correspond to YAML blocks (also overridable via environment variables such as LATTICE_APP_NAME).
//
// Key connection fields:
//   - SignalingURL (NATS signaling): auto-discovered via /api/v1/discovery from server-url; can be manually overridden with LATTICE_SIGNALING_URL / NATS_SERVICE_HOST
//   - ServerUrl (Manager API): --server-url / LATTICE_SERVER_URL / LATTICE_MANAGER_SERVICE_HOST (required)
//   - Database.DSN: falls back to local SQLite (lattice.db) by default, no extra config needed
//
// Multi-service port allocation convention (All-in-One mode):
//   - Management API  → Listen      (default :8080)
//   - Relay (TCP)     → RelayURL    (default :6266)
//   - Relay (QUIC)    → RelayQuicURL (default empty)
//   - TURN server     → Port        (default 3478)
//   - Metrics/Probe   → MetricsAddr (default :8443)
type Config struct {
	// ── Base / Runtime ───────────────────────────────────────────
	Listen        string `mapstructure:"listen"` // HTTP listen address, default :8080
	Level         string `mapstructure:"level"`  // log level
	Env           string `mapstructure:"env"`    // runtime environment: dev / prod
	Debug         bool   `mapstructure:"debug"`
	Auth          string `mapstructure:"auth"`
	AppId         string `mapstructure:"app-id"`
	Token         string `mapstructure:"token"`
	InterfaceName string `mapstructure:"interface-name"` // WireGuard interface name

	// ── Network / Address ─────────────────────────────────────────

	// SignalingURL is the NATS signaling server URL for server-side components.
	// Agent runtime: use GetSignalingURL()/SetSignalingURL() which check the
	// runtime override first (set after /api/v1/discovery).
	SignalingURL string `mapstructure:"signaling-url"`

	// runtimeNATSURL overrides SignalingURL at runtime, set by the agent after
	// auto-discovery from the server's /api/v1/discovery endpoint.
	runtimeNATSURL string

	// AuthToken is the JWT stored after 'lattice login'. Used by CLI management commands.
	AuthToken string `mapstructure:"auth-token"`

	// ServerUrl is the Manager API address.
	// The agent uses it for registration, token retrieval, status reporting, and other control-plane operations.
	// In K8s scenarios: auto-populated by environment variables like LATTICE_MANAGER_SERVICE_HOST.
	ServerUrl     string `mapstructure:"server-url"`
	RelayURL      string `mapstructure:"relay-url"`      // TCP relay connection address, default :6266
	RelayQuicURL  string `mapstructure:"relay-quic-url"` // QUIC relay connection address, empty=disabled
	TurnServerURL string `mapstructure:"stun-url"`       // TURN/STUN address
	PublicIP      string `mapstructure:"public-ip"`
	Port          int    `mapstructure:"port"`    // TURN service port, default 3478
	WgPort        int    `mapstructure:"wg-port"` // WireGuard/ICE UDP listen port, default 51820

	// ── Feature flags ─────────────────────────────────────────────
	EnableLrp    bool `mapstructure:"enable-lrp"`
	EnableTLS    bool `mapstructure:"enable-tls"`
	EnableMetric bool `mapstructure:"enable-metric"`
	EnableDNS    bool `mapstructure:"enable-dns"`
	EnableSysLog bool `mapstructure:"enable-sys-log"`
	EnableDaemon bool `mapstructure:"enable-daemon"`

	// ── Controller (Kubernetes operator) ──────────────────────────
	MetricsAddr          string `mapstructure:"metrics-addr"`
	ProbeAddr            string `mapstructure:"health-probe-bind-address"`
	EnableLeaderElection bool   `mapstructure:"leader-elect"`
	SecureMetrics        bool   `mapstructure:"metrics-secure"`
	EnableHTTP2          bool   `mapstructure:"enable-http2"`
	WebhookCertPath      string `mapstructure:"webhook-cert-path"`
	WebhookCertName      string `mapstructure:"webhook-cert-name"`
	WebhookCertKey       string `mapstructure:"webhook-cert-key"`
	MetricsCertPath      string `mapstructure:"metrics-cert-path"`
	MetricsCertName      string `mapstructure:"metrics-cert-name"`
	MetricsCertKey       string `mapstructure:"metrics-cert-key"`

	// ── Server-side nested config (YAML blocks, overridable via LATTICE_APP_*/LATTICE_DATABASE_* env vars)
	App       AppConfig       `mapstructure:"app"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Monitor   MonitorConfig   `mapstructure:"monitor"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Dex       DexConfig       `mapstructure:"dex"`
	AI        AIConfig        `mapstructure:"ai"`
}

// GetSignalingURL returns the NATS URL, checking the runtime override first.
func (c *Config) GetSignalingURL() string {
	if c.runtimeNATSURL != "" {
		return c.runtimeNATSURL
	}
	return c.SignalingURL
}

// SetSignalingURL stores the discovered NATS URL at runtime (agent-side).
func (c *Config) SetSignalingURL(url string) {
	c.runtimeNATSURL = url
}

// AIWorkflowConfig controls which write tools require human approval.
type AIWorkflowConfig struct {
	// AutoApprove maps tool name -> bool. If true, the tool executes immediately.
	// If false (default), a WorkflowRequest is created and execution waits for approval.
	// Example:
	//   auto_approve:
	//     create_peer: false
	//     delete_peer: false
	//     create_policy: false
	AutoApprove map[string]bool `mapstructure:"auto_approve"`
}

// AIConfig aggregates AI feature configuration.
// AI is a soft dependency: when Enabled=false or APIKey is empty, all /api/v1/ai/* endpoints return 503.
type AIConfig struct {
	// Enabled controls whether AI features are enabled, default false.
	// Corresponding env var: LATTICE_AI_ENABLED
	Enabled bool `mapstructure:"enabled"`

	// Provider specifies the LLM provider: anthropic (default), deepseek, openai,
	// or any OpenAI-compatible service when used with base-url.
	// Corresponding env var: LATTICE_AI_PROVIDER
	Provider string `mapstructure:"provider"`

	// APIKey is the provider's API key.
	// Corresponding env var: LATTICE_AI_API_KEY
	APIKey string `mapstructure:"api-key"`

	// Model specifies the model name; when empty, each Provider uses its built-in default.
	// Corresponding env var: LATTICE_AI_MODEL
	Model string `mapstructure:"model"`

	// BaseURL is a custom API endpoint for DeepSeek, self-hosted deployments, or API proxy.
	// Corresponding env var: LATTICE_AI_BASE_URL
	BaseURL string `mapstructure:"base-url"`

	// MaxToolCalls is the maximum number of tool calls per conversation turn, default 5.
	MaxToolCalls int `mapstructure:"max-tool-calls"`

	// AuditSchedule is the cron expression for security audit scheduling, default "0 2 * * *" (2 AM daily).
	// When empty, scheduled auditing is disabled.
	AuditSchedule string `mapstructure:"audit-schedule"`

	// Workflow controls write-tool approval behaviour.
	Workflow AIWorkflowConfig `mapstructure:"workflow"`
}

// AppConfig aggregates application-layer server-side configuration (excluding CLI override fields).
type AppConfig struct {
	Name       string        `mapstructure:"name"`
	InitAdmins []AdminConfig `mapstructure:"initAdmins"` // list of admins initialized on first startup
}

type AdminConfig struct {
	Username string `yaml:"username" mapstructure:"username"`
	Password string `yaml:"password" mapstructure:"password"`
}

// DatabaseConfig holds database connection configuration.
//
// Multi-environment DSN strategy:
//   - DSN is empty (default) → Driver automatically set to "sqlite", db.NewStore uses lattice.db; suitable for development/open-source.
//   - DSN contains @tcp( / mysql:// / mariadb:// → Driver auto-detected as "mariadb", MySQL-compatible protocol.
//   - Can be explicitly overridden via LATTICE_DATABASE_DSN / LATTICE_DATABASE_DRIVER environment variables.
type DatabaseConfig struct {
	Driver string `mapstructure:"driver"`
	DSN    string `mapstructure:"dsn"`
}

type NatsConfig struct {
	URL string `mapstructure:"url"`
}

type MonitorConfig struct {
	Address     string `mapstructure:"address"`
	TemplateDir string `mapstructure:"templateDir"` // YAML template path, default "config/metrics/templates.yaml"
}

// TelemetryConfig configures the lightweight VM telemetry push module in the agent.
type TelemetryConfig struct {
	// VMEndpoint is the VictoriaMetrics remote write URL, e.g. "http://vm:8428/api/v1/write".
	// Push is disabled when empty.
	VMEndpoint string `mapstructure:"vmEndpoint"`
	// IntervalSeconds is the push interval in seconds. Defaults to 30.
	IntervalSeconds int `mapstructure:"intervalSeconds"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type DexConfig struct {
	Issur       string   `mapstructure:"issur"`
	ProviderUrl string   `mapstructure:"providerUrl"`
	AdminEmails []string `mapstructure:"adminEmails"` // email allowlist for auto-granting platform_admin role to Dex login users
}

// NetworkOptions holds option parameters for network operations.
type NetworkOptions struct {
	AppId      string
	Identifier string
	Name       string
	CIDR       string
	ServerUrl  string
}

// ─────────────────────────────────────────────
//
//	Pre-flight validation
//
// ─────────────────────────────────────────────

// ValidateConfig is deprecated; use ValidateAndReport(cfg, false) instead.
// Kept for backward compatibility with existing call sites.
func ValidateConfig(cfg *Config) error {
	return ValidateAndReport(cfg, false)
}

// ─────────────────────────────────────────────
//
//	K8s service discovery & database driver inference
//
// ─────────────────────────────────────────────

// applyK8sFallbacks attempts to auto-populate key fields that are still empty after Unmarshal
// using K8s standard service environment variables (*_SERVICE_HOST / *_SERVICE_PORT).
//
// Its priority is below all "onion model" layers, serving only as a last resort.
//
// K8s service discovery principle: when a Service is deployed, K8s injects into Pods in the same namespace:
//
//	<SERVICE_NAME>_SERVICE_HOST=<ClusterIP>
//	<SERVICE_NAME>_SERVICE_PORT=<Port>
//
// Dashes in the Service name are replaced with underscores and the entire name is uppercased.
// For example: Service "nats" → NATS_SERVICE_HOST; Service "lattice-manager" → LATTICE_MANAGER_SERVICE_HOST.
func applyK8sFallbacks(cfg *Config) {
	// ── Manager API address: check env vars for common Service names in order ──
	if cfg.ServerUrl == "" {
		for _, prefix := range []string{
			"LATTICE_MANAGER", // Service: lattice-manager
			"LATTICE_API",     // Service: lattice-api
			"MANAGER",         // Service: manager
		} {
			if host := os.Getenv(prefix + "_SERVICE_HOST"); host != "" {
				port := os.Getenv(prefix + "_SERVICE_PORT")
				if port == "" {
					port = "8080"
				}
				cfg.ServerUrl = "http://" + host + ":" + port
				log.Info("K8s service discovery", "source", prefix+"_SERVICE_HOST", "server-url", cfg.ServerUrl)
				break
			}
		}
	}
}

// inferDatabaseDriver infers the database driver from the DSN content, for cases where the user has not explicitly specified a driver.
//
// Inference rules:
//   - DSN is empty → driver="sqlite" (db.NewStore automatically uses lattice.db, zero extra dependencies)
//   - DSN contains @tcp( or prefix mysql:// / mariadb:// → driver="mariadb" (MySQL-compatible protocol)
//   - Other DSN formats (file:, *.db paths, etc.) → driver="sqlite"
//
// If the driver has been explicitly set by the user to a non-sqlite value, inference is skipped and the user's intent is respected.
func inferDatabaseDriver(cfg *Config) {
	db := &cfg.Database

	if db.DSN == "" {
		// No DSN → open-source/dev default: SQLite, db.NewStore uses "lattice.db"
		db.Driver = "sqlite"
		return
	}

	// driver was explicitly set by user to a different value → respect it, do not override
	if db.Driver != "" && db.Driver != "sqlite" {
		return
	}

	// Infer driver from DSN format
	dsn := db.DSN
	switch {
	case strings.Contains(dsn, "@tcp("),
		strings.HasPrefix(dsn, "mysql://"),
		strings.HasPrefix(dsn, "mariadb://"):
		db.Driver = "mariadb"
		log.Info("inferred database driver from DSN", "driver", "mariadb")
	default:
		db.Driver = "sqlite"
	}
}

// ─────────────────────────────────────────────
//
//	Path helpers
//
// ─────────────────────────────────────────────

// GetConfigFilePath returns the main config file path (backward compatible).
func GetConfigFilePath() string { return GetManager().dir + "/lattice.yaml" }

// peekConfigDir retrieves the config directory early, before full loading, with priority:
// --config-dir > LATTICE_CONFIG_DIR > ~/.lattice
func peekConfigDir(cmd *cobra.Command) string {
	if f := cmd.Flags().Lookup("config-dir"); f != nil && f.Changed {
		return f.Value.String()
	}
	if dir := os.Getenv("LATTICE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	if home == "/" {
		return "/etc/lattice"
	}
	return home + "/.lattice"
}

// ─────────────────────────────────────────────
//
//	Internal helpers
//
// ─────────────────────────────────────────────

// setDefaults sets hard-coded default values for configuration items (the bottom layer of the onion model).
//
// Design principles:
//   - Fields with universally reasonable values (listen, level, port numbers, etc.) → set concrete defaults.
//   - Connection addresses that are strongly environment-dependent → default to empty, forcing the user to
//     explicitly configure them or rely on K8s service discovery auto-injection.
//     This avoids the risk of "using the wrong environment" (e.g., accidentally connecting to a production MariaDB).
func setDefaults(v *viper.Viper) {
	v.SetDefault("listen", ":8080")
	v.SetDefault("level", "info")
	v.SetDefault("env", "dev")

	// Critical connection addresses: no hard-coded defaults, empty means "not configured":
	//   server-url    = ""  → Manager API unknown (agent-side ValidateConfig reports error)
	//   database.dsn  = ""  → auto-falls back to local SQLite lattice.db (handled by inferDatabaseDriver)
	v.SetDefault("auth-token", "")
	v.SetDefault("server-url", "")

	v.SetDefault("stun-url", "stun.alattice.io:3478")
	v.SetDefault("relay-url", ":6266")
	v.SetDefault("relay-quic-url", "")
	v.SetDefault("port", 3478)
	v.SetDefault("wg-port", 51820)

	// database.driver defaults to sqlite, which together with database.dsn="" provides out-of-the-box local storage.
	// If the user provides a MySQL/MariaDB DSN, inferDatabaseDriver() automatically corrects the driver to "mariadb".
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "")

	v.SetDefault("dex.providerUrl", "") // empty = disable Dex OIDC
	v.SetDefault("monitor.address", "")

	v.SetDefault("metrics-addr", ":8443")
	v.SetDefault("health-probe-bind-address", ":8081")

	v.SetDefault("ai.enabled", false)
	v.SetDefault("ai.provider", "anthropic")
	v.SetDefault("ai.model", "")
	v.SetDefault("ai.max-tool-calls", 5)
	v.SetDefault("ai.audit-schedule", "0 2 * * *")

	v.SetDefault("app.name", "Lattice")
	v.SetDefault("app.initAdmins", []map[string]string{
		{"username": "admin", "password": "123456"},
	})
}

// peekEnv retrieves the environment early, before full loading, for selecting the environment config file.
func peekEnv(cmd *cobra.Command) string {
	if f := cmd.Flags().Lookup("env"); f != nil && f.Changed {
		return f.Value.String()
	}
	if e := os.Getenv("LATTICE_ENV"); e != "" {
		return e
	}
	return "dev"
}
