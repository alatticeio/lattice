package models

import (
	"time"
)

const (
	ConfigKeyNatsURL = "nats_url"
	ConfigKeyStunURL = "stun_url"
)

// Feature flag keys stored in la_system_config with "feature." prefix.
const (
	FeatureKeyAIAssistant  = "feature.ai_assistant"
	FeatureKeyAgentSandbox = "feature.agent_sandbox"
	FeatureKeyAlerts       = "feature.alerts"
	FeatureKeyMonitor      = "feature.monitor"
	FeatureKeyMCPServers   = "feature.mcp_servers"
	FeatureKeyRelays       = "feature.relays"
	FeatureKeyNetworkPeer  = "feature.network_peering"
	FeatureKeyClusterPeer  = "feature.cluster_peering"
	FeatureKeyApprovals    = "feature.approvals"
)

// FeatureFlagDef declares a feature flag's metadata. The list is code-authoritative;
// the DB only stores the enabled/disabled state.
type FeatureFlagDef struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Group   string `json:"group"`
	Default bool   `json:"default"`
}

// FeatureFlagDefs is the exhaustive list of feature flags managed by the platform.
var FeatureFlagDefs = []FeatureFlagDef{
	{Key: FeatureKeyAIAssistant, Label: "AI Assistant", Group: "ai", Default: true},
	{Key: FeatureKeyMCPServers, Label: "MCP Servers", Group: "ai", Default: true},
	{Key: FeatureKeyAgentSandbox, Label: "Agent Sandbox", Group: "sandbox", Default: true},
	{Key: FeatureKeyAlerts, Label: "Alerts", Group: "settings", Default: true},
	{Key: FeatureKeyRelays, Label: "Relays", Group: "settings", Default: true},
	{Key: FeatureKeyMonitor, Label: "Monitor", Group: "workspace", Default: true},
	{Key: FeatureKeyNetworkPeer, Label: "Network Peering", Group: "platform", Default: true},
	{Key: FeatureKeyClusterPeer, Label: "Cluster Peering", Group: "platform", Default: true},
	{Key: FeatureKeyApprovals, Label: "Approvals", Group: "platform", Default: true},
}

type SystemConfig struct {
	Key       string    `gorm:"primaryKey;type:varchar(128);not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemConfig) TableName() string { return "la_system_config" }
