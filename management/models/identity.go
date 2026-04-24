package models

import "time"

// UserIdentity links a platform User to an external identity provider account.
// A single User may have multiple identities (e.g. local + GitHub + corporate OIDC).
type UserIdentity struct {
	Model
	UserID     string    `gorm:"index;not null" json:"userId"`
	Provider   string    `gorm:"size:50;not null" json:"provider"` // "local" | "dex" | "ldap" | "github"
	ExternalID string    `gorm:"not null" json:"externalId"`        // subject from IdP
	Email      string    `json:"email"`
	Metadata   string    `gorm:"type:text" json:"metadata,omitempty"` // JSON raw claims
	LastSyncAt time.Time `json:"lastSyncAt"`
}

func (UserIdentity) TableName() string { return "t_user_identity" }
