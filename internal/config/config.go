// Copyright 2025 The Wireflow Authors, Inc.
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

// Package config 实现统一的配置中心，采用"洋葱模型"加载优先级：
//
//	默认值 < wireflow.yaml < wireflow.{env}.yaml < 环境变量(WIREFLOW_*) < 命令行参数
//	                                               < K8s 服务发现兜底（仅空值生效）
//
// 推荐调用方式（在 cmd 的 PersistentPreRunE 中）：
//
//	if err := config.GetManager().Load(cmd); err != nil { return err }
//
// 加载完成后通过 config.GlobalConfig 或 config.Conf 访问（指向同一对象）。
// 在需要校验关键连接地址的服务入口，额外调用 config.ValidateAndReport(config.GlobalConfig, isServer)。
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	wflog "wireflow/internal/log"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var log = wflog.GetLogger("config")

// ─────────────────────────────────────────────
//
//	全局单例
//
// ─────────────────────────────────────────────

var (
	managerOnce   sync.Once
	globalManager *ConfigManager

	// GlobalConfig / Conf 指向同一个 Config 对象，加载后即为唯一真理来源。
	// 服务端组件推荐使用 GlobalConfig；既有扁平字段调用方直接用 Conf。
	GlobalConfig = &Config{}
	Conf         = GlobalConfig
)

// GetManager 返回全局唯一的 ConfigManager（线程安全）。
func GetManager() *ConfigManager {
	managerOnce.Do(func() {
		globalManager = &ConfigManager{v: viper.New()}
	})
	return globalManager
}

// NewConfigManager 是 GetManager 的向后兼容别名。
func NewConfigManager() *ConfigManager { return GetManager() }

// ─────────────────────────────────────────────
//
//	ConfigManager
//
// ─────────────────────────────────────────────

// ConfigManager 封装 Viper 实例，提供多环境加载能力。
type ConfigManager struct {
	v    *viper.Viper
	once sync.Once
	dir  string // 解析后的配置目录（由 --config-dir / WIREFLOW_CONFIG_DIR / 默认值决定）
}

// Viper 暴露底层实例，供需要精细控制的调用方使用。
func (cm *ConfigManager) Viper() *viper.Viper { return cm.v }

// Load 按"洋葱模型"加载配置，只执行一次（幂等）。
//
//  1. 硬编码默认值
//  2. wireflow.yaml（基础配置）
//  3. wireflow.{env}.yaml（环境差异配置，MergeInConfig）
//  4. 环境变量（WIREFLOW_ 前缀）
//  5. 命令行参数（BindPFlags，最高优先级）
//  6. K8s 服务发现兜底（仅对仍为空的字段生效）
//  7. 数据库驱动推断
//
// --save：将本次命令行显式指定的参数合并持久化到配置文件（不覆盖其他已有项）。
func (cm *ConfigManager) Load(cmd *cobra.Command) error {
	var err error
	cm.once.Do(func() { err = cm.load(cmd) })
	return err
}

// LoadConf 是 Load 的别名，保持向后兼容。
func (cm *ConfigManager) LoadConf(cmd *cobra.Command) error { return cm.Load(cmd) }

// Save 将当前内存配置（含所有层的合并结果）写回配置文件。
func (cm *ConfigManager) Save() error {
	path := cm.dir + "/wireflow.yaml"
	if err := cm.v.WriteConfig(); err != nil {
		return cm.v.WriteConfigAs(path)
	}
	return nil
}

// SaveChangedFlags 仅将本次命令行显式指定（Changed）的 flag 合并写入配置文件，
// 不覆盖文件中已有的其他配置项，也不写入未被用户显式设置的默认值。
//
// 对比 Save()：Save() 写入 Viper 全部已知键（含默认值）；
// SaveChangedFlags() 只写入本次 --xxx 参数中实际使用的键。
func (cm *ConfigManager) SaveChangedFlags(cmd *cobra.Command) error {
	path := cm.dir + "/wireflow.yaml"
	if err := os.MkdirAll(cm.dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// 将命令行显式指定的 flag 注入 Viper（跳过 --save 本身）
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if f.Name == "save" {
			return
		}
		cm.v.Set(f.Name, cm.v.Get(f.Name))
	})

	// WriteConfig 写入 Viper 当前已 Set 的键（含从配置文件读取的已有项），
	// 不写入仅通过 SetDefault 设置但未被任何层覆盖的默认值。
	if err := cm.v.WriteConfig(); err != nil {
		return cm.v.WriteConfigAs(path)
	}
	return nil
}

func (cm *ConfigManager) load(cmd *cobra.Command) error {
	v := cm.v
	v.SetConfigType("yaml")

	// ── 第一层：默认值 ────────────────────────────────────────────
	setDefaults(v)

	// ── 确定配置目录（提前 peek）────────────────────────────────
	cm.dir = peekConfigDir(cmd)

	// ── 确定运行环境（提前 peek，不依赖完整加载）────────────────
	env := peekEnv(cmd)

	// ── 第二层：wireflow.yaml ─────────────────────────────────────
	baseFile := cm.dir + "/wireflow.yaml"
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

	// ── 第三层：wireflow.{env}.yaml ──────────────────────────────
	envFile := fmt.Sprintf("%s/wireflow.%s.yaml", cm.dir, env)
	v.SetConfigFile(envFile)
	if _, err := os.Stat(envFile); err == nil {
		if err := v.MergeInConfig(); err != nil {
			log.Warn("failed to merge env config", "file", envFile, "err", err)
		} else {
			log.Info("env config loaded", "file", envFile)
		}
	}
	// 重置回 baseFile，确保后续 WriteConfig / Save 写入正确路径
	v.SetConfigFile(baseFile)

	// ── 第四层：环境变量（WIREFLOW_APP_LISTEN, WIREFLOW_SIGNALING_URL …）
	v.SetEnvPrefix("WIREFLOW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// ── 第五层：命令行参数（BindPFlags 自动全量绑定）────────────
	if err := v.BindPFlags(cmd.Flags()); err != nil {
		return fmt.Errorf("[config] BindPFlags 失败: %w", err)
	}

	// ── Unmarshal：GlobalConfig 与 Conf 共享同一指针 ─────────────
	if err := v.Unmarshal(GlobalConfig); err != nil {
		return fmt.Errorf("[config] Unmarshal 失败: %w", err)
	}
	Conf = GlobalConfig

	// ── 兼容旧版 vm-endpoint key（kebab-case → vmEndpoint）──────
	if GlobalConfig.Telemetry.VMEndpoint == "" {
		if oldEp := v.GetString("vm-endpoint"); oldEp != "" {
			GlobalConfig.Telemetry.VMEndpoint = oldEp
		}
	}

	// ── 第六层：K8s 服务发现兜底（仅对仍为空的字段生效）─────────
	applyK8sFallbacks(GlobalConfig)

	// ── 数据库驱动推断（从 DSN 格式自动识别驱动类型）────────────
	inferDatabaseDriver(GlobalConfig)

	log.Debug("config loaded", "env", env, "listen", GlobalConfig.Listen, "driver", GlobalConfig.Database.Driver)

	// ── --save：把本次命令行显式指定的参数持久化回配置文件 ───────
	// 典型用法：wireflow up --signaling-url nats://x:4222 --server-url http://y --save
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
//	Config：唯一的配置结构体
//
// ─────────────────────────────────────────────

// Config 是整个项目的统一配置结构体。
//
// 顶层扁平字段对应 CLI flag 名（BindPFlags 直接映射）；
// 嵌套子结构体对应 YAML 中的块（也可通过 WIREFLOW_APP_NAME 等环境变量覆盖）。
//
// 三个关键连接字段均无硬编码默认值，必须由用户显式提供：
//   - SignalingURL（NATS 信令，即 nats_url）：--signaling-url / WIREFLOW_SIGNALING_URL / NATS_SERVICE_HOST
//   - ServerUrl  （Manager API，即 manager_api_url）：--server-url / WIREFLOW_SERVER_URL / WIREFLOW_MANAGER_SERVICE_HOST
//   - Database.DSN：缺省时自动退化为本地 SQLite（wireflow.db），无需额外配置
//
// 多子服务端口分配约定（All-in-One 模式）：
//   - Management API  → Listen      (默认 :8080)
//   - Wrrper relay    → WrrperURL   (默认 :6266)
//   - TURN server     → Port        (默认 3478)
//   - Metrics/Probe   → MetricsAddr (默认 :8443)
type Config struct {
	// ── 基础 / 运行时 ─────────────────────────────────────────────
	Listen        string `mapstructure:"listen"` // HTTP 监听地址，默认 :8080
	Level         string `mapstructure:"level"`  // 日志级别
	Env           string `mapstructure:"env"`    // 运行环境：dev / prod
	Debug         bool   `mapstructure:"debug"`
	Auth          string `mapstructure:"auth"`
	AppId         string `mapstructure:"app-id"`
	Token         string `mapstructure:"token"`
	InterfaceName string `mapstructure:"interface-name"` // WireGuard 接口名

	// ── 网络 / 地址 ───────────────────────────────────────────────

	// SignalingURL 是 NATS 信令服务地址（对应需求中的 nats_url）。
	// 空值含义：信令服务不可用——server 端降级为 noop，agent 端 ValidateAndReport 会拒绝启动。
	// K8s 场景：部署名为 "nats" 的 Service 后，K8s 会注入 NATS_SERVICE_HOST，由
	// applyK8sFallbacks() 自动补全本字段，无需手动配置。
	SignalingURL string `mapstructure:"signaling-url"`

	// ServerUrl 是 Manager API 地址（对应需求中的 manager_api_url）。
	// agent 用于注册、获取 Token、上报状态等控制面操作。
	// K8s 场景：由 WIREFLOW_MANAGER_SERVICE_HOST 等环境变量自动补全。
	ServerUrl     string `mapstructure:"server-url"`
	WrrperURL     string `mapstructure:"wrrper-url"`   // Wrrper relay 地址，默认 :6266
	WrrpQuicURL   string `mapstructure:"wrrp-quic-url"` // QUIC relay server address
	TurnServerURL string `mapstructure:"stun-url"`   // TURN/STUN 地址
	PublicIP      string `mapstructure:"public-ip"`
	Port          int    `mapstructure:"port"`    // TURN 业务端口，默认 3478
	WgPort        int    `mapstructure:"wg-port"` // WireGuard/ICE UDP 监听端口，默认 51820

	// ── 功能开关 ──────────────────────────────────────────────────
	EnableWrrp   bool `mapstructure:"enable-wrrp"`
	EnableTLS    bool `mapstructure:"enable-tls"`
	EnableMetric bool `mapstructure:"enable-metric"`
	EnableDNS    bool `mapstructure:"enable-dns"`
	EnableSysLog bool `mapstructure:"enable-sys-log"`
	EnableDaemon bool `mapstructure:"enable-daemon"`

	// ── Controller（Kubernetes operator）──────────────────────────
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

	// ── 服务端嵌套配置（YAML 块，环境变量 WIREFLOW_APP_*/WIREFLOW_DATABASE_* 覆盖）
	App       AppConfig       `mapstructure:"app"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Monitor   MonitorConfig   `mapstructure:"monitor"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Dex       DexConfig       `mapstructure:"dex"`
}

// AppConfig 聚合应用层服务端配置（不含 CLI 覆盖字段）。
type AppConfig struct {
	Name       string        `mapstructure:"name"`
	InitAdmins []AdminConfig `mapstructure:"initAdmins"` // 首次启动时初始化的管理员列表
}

type AdminConfig struct {
	Username string `yaml:"username" mapstructure:"username"`
	Password string `yaml:"password" mapstructure:"password"`
}

// DatabaseConfig 数据库连接配置。
//
// 多环境 DSN 策略（对应需求中的 database_dsn 逻辑）：
//   - DSN 为空（默认）→ Driver 自动设为 "sqlite"，db.NewStore 使用 wireflow.db；适合开发/开源场景。
//   - DSN 含 @tcp( / mysql:// / mariadb:// → Driver 自动推断为 "mariadb"，兼容 MySQL 协议。
//   - 可通过 WIREFLOW_DATABASE_DSN / WIREFLOW_DATABASE_DRIVER 环境变量显式覆盖。
type DatabaseConfig struct {
	Driver string `mapstructure:"driver"`
	DSN    string `mapstructure:"dsn"`
}

type NatsConfig struct {
	URL string `mapstructure:"url"`
}

type MonitorConfig struct {
	Address string `mapstructure:"address"`
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
	Issur       string `mapstructure:"issur"`
	ProviderUrl string `mapstructure:"providerUrl"`
}

// NetworkOptions 用于网络操作的选项参数。
type NetworkOptions struct {
	AppId      string
	Identifier string
	Name       string
	CIDR       string
	ServerUrl  string
}

// ─────────────────────────────────────────────
//
//	Pre-flight 校验
//
// ─────────────────────────────────────────────

// ValidateConfig is deprecated; use ValidateAndReport(cfg, false) instead.
// Kept for backward compatibility with existing call sites.
func ValidateConfig(cfg *Config) error {
	return ValidateAndReport(cfg, false)
}

// ─────────────────────────────────────────────
//
//	K8s 服务发现 & 数据库驱动推断
//
// ─────────────────────────────────────────────

// applyK8sFallbacks 在 Unmarshal 之后，对仍为空的关键字段尝试通过
// K8s 标准服务环境变量（*_SERVICE_HOST / *_SERVICE_PORT）自动补全。
//
// 优先级低于所有"洋葱模型"层次，仅作最后兜底。
//
// K8s 服务发现原理：部署 Service 时，K8s 会向同命名空间的 Pod 注入：
//
//	<SERVICE_NAME>_SERVICE_HOST=<ClusterIP>
//	<SERVICE_NAME>_SERVICE_PORT=<Port>
//
// 其中 Service 名中的 "-" 替换为 "_" 并全部大写。
// 例如：Service "nats" → NATS_SERVICE_HOST；Service "wireflow-manager" → WIREFLOW_MANAGER_SERVICE_HOST。
func applyK8sFallbacks(cfg *Config) {
	// ── NATS 信令地址 ─────────────────────────────────────────────
	if cfg.SignalingURL == "" {
		if host := os.Getenv("NATS_SERVICE_HOST"); host != "" {
			port := os.Getenv("NATS_SERVICE_PORT")
			if port == "" {
				port = "4222"
			}
			cfg.SignalingURL = "nats://" + host + ":" + port
			log.Info("K8s service discovery: NATS_SERVICE_HOST", "signaling-url", cfg.SignalingURL)
		}
	}

	// ── Manager API 地址：依次检测常见 Service 名对应的环境变量 ──
	if cfg.ServerUrl == "" {
		for _, prefix := range []string{
			"WIREFLOW_MANAGER", // Service: wireflow-manager
			"WIREFLOW_API",     // Service: wireflow-api
			"MANAGER",          // Service: manager
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

// inferDatabaseDriver 根据 DSN 内容推断数据库驱动，处理用户未显式指定 driver 的场景。
//
// 推断规则：
//   - DSN 为空 → driver="sqlite"（db.NewStore 自动使用 wireflow.db，零额外依赖）
//   - DSN 含 @tcp( 或前缀 mysql:// / mariadb:// → driver="mariadb"（MySQL 兼容协议）
//   - 其他 DSN 格式（file:、*.db 路径等）→ driver="sqlite"
//
// 若 driver 已被用户显式设置为非 sqlite 的值，则跳过推断，尊重用户意图。
func inferDatabaseDriver(cfg *Config) {
	db := &cfg.Database

	if db.DSN == "" {
		// 无 DSN → 开源/开发默认：SQLite，db.NewStore 使用 "wireflow.db"
		db.Driver = "sqlite"
		return
	}

	// driver 已被用户显式设置为其他驱动 → 尊重，不覆盖
	if db.Driver != "" && db.Driver != "sqlite" {
		return
	}

	// 从 DSN 格式推断驱动
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
//	路径辅助
//
// ─────────────────────────────────────────────

// GetConfigFilePath 返回主配置文件路径（向后兼容）。
func GetConfigFilePath() string { return GetManager().dir + "/wireflow.yaml" }

// peekConfigDir 在完整加载之前提前获取配置目录，优先级：
// --config-dir > WIREFLOW_CONFIG_DIR > ~/.wireflow
func peekConfigDir(cmd *cobra.Command) string {
	if f := cmd.Flags().Lookup("config-dir"); f != nil && f.Changed {
		return f.Value.String()
	}
	if dir := os.Getenv("WIREFLOW_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	if home == "/" {
		return "/etc/wireflow"
	}
	return home + "/.wireflow"
}

// ─────────────────────────────────────────────
//
//	内部辅助
//
// ─────────────────────────────────────────────

// setDefaults 设置各配置项的硬编码默认值（洋葱模型最底层）。
//
// 设计原则：
//   - 有通用合理值的字段（listen、level、端口号等）→ 设置具体默认值。
//   - 与具体环境强相关的连接地址 → 默认为空，强制用户显式配置或通过 K8s 服务发现自动注入。
//     这样可避免"用错环境"的隐患（如误连到生产 K8s 集群的 MariaDB）。
func setDefaults(v *viper.Viper) {
	v.SetDefault("listen", ":8080")
	v.SetDefault("level", "info")
	v.SetDefault("env", "dev")

	// 关键连接地址：不设硬编码默认值，空值即"未配置"语义：
	//   signaling-url = ""  → 信令服务不可用（server 端降级，agent 端 ValidateConfig 报错）
	//   server-url    = ""  → Manager API 未知（agent 端 ValidateConfig 报错）
	//   database.dsn  = ""  → 自动退化为本地 SQLite wireflow.db（inferDatabaseDriver 处理）
	v.SetDefault("signaling-url", "")
	v.SetDefault("server-url", "")

	v.SetDefault("stun-url", "stun.wireflow.run:3478")
	v.SetDefault("wrrper-url", ":6266")
	v.SetDefault("wrrp-quic-url", "")
	v.SetDefault("port", 3478)
	v.SetDefault("wg-port", 51820)

	// database.driver 默认 sqlite，与 database.dsn="" 配合实现开箱即用的本地存储。
	// 若用户提供了 MySQL/MariaDB DSN，inferDatabaseDriver() 会自动将 driver 修正为 "mariadb"。
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "")

	v.SetDefault("dex.providerUrl", "") // 空 = 禁用 Dex OIDC
	v.SetDefault("monitor.address", "")

	v.SetDefault("metrics-addr", ":8443")
	v.SetDefault("health-probe-bind-address", ":8081")

	v.SetDefault("app.name", "WireFlow")
	v.SetDefault("app.initAdmins", []map[string]string{
		{"username": "admin", "password": "123456"},
	})
}

// peekEnv 在完整加载之前提前获取 env，用于选择环境配置文件。
func peekEnv(cmd *cobra.Command) string {
	if f := cmd.Flags().Lookup("env"); f != nil && f.Changed {
		return f.Value.String()
	}
	if e := os.Getenv("WIREFLOW_ENV"); e != "" {
		return e
	}
	return "dev"
}
