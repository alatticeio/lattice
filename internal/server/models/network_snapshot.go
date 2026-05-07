package models

import "time"

// NetworkSnapshot is a point-in-time capture of all network objects in a workspace.
// Triggered by CRD change events. Stored as serialized JSON.
// Retention: 90 days community, 1 year Pro.
type NetworkSnapshot struct {
	ID          string    `gorm:"primaryKey;type:varchar(36);not null" json:"id"`
	WorkspaceID string    `gorm:"size:36;index:idx_snap_ws_time"       json:"workspaceId"`
	Namespace   string    `gorm:"size:200"                             json:"namespace"`
	CapturedAt  time.Time `gorm:"index:idx_snap_ws_time"               json:"capturedAt"`
	// TriggerType is what caused this snapshot.
	// Values: policy_change | peer_online | peer_offline | workflow_executed | manual | scheduled
	TriggerType string `gorm:"size:30;index"  json:"triggerType"`
	TriggerBy   string `gorm:"size:100"       json:"triggerBy"` // user ID or "system"

	// Serialized JSON arrays of CRD objects (semantic fields only, no managedFields).
	Peers    string `gorm:"type:text" json:"peers"`
	Policies string `gorm:"type:text" json:"policies"`
	Networks string `gorm:"type:text" json:"networks"`
	Presence string `gorm:"type:text" json:"presence"`
}

func (NetworkSnapshot) TableName() string { return "t_network_snapshot" }
