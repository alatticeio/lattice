package models

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// RefreshToken represents a long-lived refresh token stored as SHA256 hash.
// The raw token value is only shown once upon creation and is never stored.
type RefreshToken struct {
	Model
	UserID    string     `gorm:"index;size:64" json:"userId"`
	TokenHash string     `gorm:"uniqueIndex;size:64" json:"-"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

func (RefreshToken) TableName() string {
	return "t_refresh_token"
}

// HashRefreshToken computes a SHA256 hex digest of the raw refresh token string.
func HashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
