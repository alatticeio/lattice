package models

import "time"

// PeerIdentity stores stable logical identities for devices in a network.
// Policies reference PeerIdentity names instead of IPs, decoupling security from network topology.
type PeerIdentity struct {
	Model

	NetworkID            string     `gorm:"size:36;uniqueIndex:idx_pi_network_name;not null" json:"network_id"`
	Name                 string     `gorm:"size:200;uniqueIndex:idx_pi_network_name;not null" json:"name"`
	PeerRef              string     `gorm:"size:200;not null" json:"peer_ref"`
	PreviousPeerRef      string     `gorm:"size:200" json:"previous_peer_ref,omitempty"`
	GracePeriodSeconds   int        `gorm:"default:300" json:"grace_period_seconds"`
	ResolvedPeerIP       string     `gorm:"size:64" json:"resolved_peer_ip,omitempty"`
	PreviousPeerIP       string     `gorm:"size:64" json:"previous_peer_ip,omitempty"`
	GracePeriodExpiresAt *time.Time `json:"grace_period_expires_at,omitempty"`
	Description          string     `gorm:"size:500" json:"description,omitempty"`
}

func (PeerIdentity) TableName() string { return "t_peer_identity" }
