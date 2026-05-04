package license

import "time"

// LicenseType represents the tier of a license.
type LicenseType string

const (
	Trial      LicenseType = "trial"
	Standard   LicenseType = "standard"
	Enterprise LicenseType = "enterprise"
	NFR        LicenseType = "nfr"
)

// License represents a parsed and verified license.
type License struct {
	JTI          string        `json:"jti"`
	Subject      string        `json:"sub"`
	Issuer       string        `json:"iss"`
	IssuedAt     time.Time     `json:"iat"`
	ExpiresAt    time.Time     `json:"exp"`
	CustomerName string        `json:"customer_name"`
	Type         LicenseType   `json:"type"`
	Features     []string      `json:"features"`
	Limits       LicenseLimits `json:"limits"`
	PublicKeyID  string        `json:"public_key_id"`
	Raw          string
}

// LicenseLimits defines resource limits imposed by a license.
type LicenseLimits struct {
	MaxNodes    int `json:"max_nodes"`
	MaxClusters int `json:"max_clusters"`
}

// Status represents the current state of a license.
type Status string

const (
	StatusValid    Status = "valid"
	StatusExpired  Status = "expired"
	StatusInvalid  Status = "invalid"
	StatusNotFound Status = "not_found"
	StatusRevoked  Status = "revoked"
)

// GracePeriod returns the grace period duration for a license type.
func (lt LicenseType) GracePeriod() time.Duration {
	switch lt {
	case Standard:
		return 7 * 24 * time.Hour
	case Enterprise:
		return 14 * 24 * time.Hour
	case NFR:
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}
