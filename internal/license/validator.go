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

package license

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// trustedPublicKeys stores known public keys indexed by key ID.
// pk-2026-01 is the initial signing key.
// On key rotation, add new entries here; old keys remain to validate existing licenses.
var trustedPublicKeys = map[string]string{
	"pk-2026-01": "", // Set via init() in keys.go
}

// proVerifier performs Ed25519 JWT-based license verification.
type proVerifier struct {
	cached   *License
	cachedAt time.Time
	cacheTTL time.Duration
	tier     string
}

// communityVerifier always returns StatusNotFound — no features available.
type communityVerifier struct{}

// NewVerifier creates a tier-aware license verifier.
// For "pro" tier: tries license file first, falls back to tier-based (all features granted).
// For "community" tier: always returns false for all features.
func NewVerifier(tier string) Verifier {
	if tier == "pro" {
		return &proVerifier{cacheTTL: 5 * time.Minute, tier: tier}
	}
	return &communityVerifier{}
}

func (v *communityVerifier) Verify() (*License, Status, error) {
	return nil, StatusNotFound, fmt.Errorf("licensing is a Pro feature")
}

func (v *communityVerifier) HasFeature(_ string) bool {
	return false
}

func (v *proVerifier) Verify() (*License, Status, error) {
	// Return cached result if fresh
	if v.cached != nil && time.Since(v.cachedAt) < v.cacheTTL {
		return v.cached, StatusValid, nil
	}

	path, fromEnv, found := ResolvePath()
	if !found {
		return nil, StatusNotFound, fmt.Errorf("no license file found. Install with: lattice license install ./license.lic")
	}

	var rawJWT string
	if fromEnv {
		rawJWT = path
	} else {
		data, err := ReadFile(path)
		if err != nil {
			return nil, StatusNotFound, err
		}
		rawJWT = data
	}

	// Parse and verify the JWT
	token, err := jwt.Parse(rawJWT, v.keyFunc)
	if err != nil {
		return nil, StatusInvalid, fmt.Errorf("license verification failed: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, StatusInvalid, fmt.Errorf("license verification failed: invalid claims")
	}

	lic := parseClaims(claims, rawJWT)

	// Check expiration
	if time.Now().After(lic.ExpiresAt) {
		grace := lic.Type.GracePeriod()
		if time.Since(lic.ExpiresAt) > grace {
			return lic, StatusExpired, fmt.Errorf("license expired on %s", lic.ExpiresAt.Format(time.RFC3339))
		}
	}

	v.cached = lic
	v.cachedAt = time.Now()
	return lic, StatusValid, nil
}

func (v *proVerifier) HasFeature(feature string) bool {
	// Try license file first.
	lic, status, _ := v.Verify()
	if status == StatusValid && lic != nil {
		for _, f := range lic.Features {
			if f == feature {
				return true
			}
		}
	}
	// Fall back to tier-based: pro tier grants all features.
	return v.tier == "pro"
}

func (v *proVerifier) keyFunc(token *jwt.Token) (interface{}, error) {
	// Require EdDSA (Ed25519) algorithm
	if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}

	// Look up the public key by key ID
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("license missing 'kid' header")
	}

	pubKeyPEM, ok := trustedPublicKeys[kid]
	if !ok || pubKeyPEM == "" {
		return nil, fmt.Errorf("license signed with unknown key: %s", kid)
	}

	// The public key is set at build time via ldflags or embedded in the binary
	// For now, return nil to indicate the key needs to be embedded
	if pubKeyPEM == "" {
		return nil, fmt.Errorf("public key %s is not embedded in this binary", kid)
	}

	key, err := jwt.ParseEdPublicKeyFromPEM([]byte(pubKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse public key %s: %w", kid, err)
	}
	return key, nil
}

func parseClaims(c jwt.MapClaims, raw string) *License {
	lic := &License{Raw: raw}

	if v, ok := c["jti"].(string); ok {
		lic.JTI = v
	}
	if v, ok := c["sub"].(string); ok {
		lic.Subject = v
	}
	if v, ok := c["iss"].(string); ok {
		lic.Issuer = v
	}
	if v, ok := c["customer_name"].(string); ok {
		lic.CustomerName = v
	}
	if v, ok := c["type"].(string); ok {
		lic.Type = LicenseType(v)
	}
	if v, ok := c["public_key_id"].(string); ok {
		lic.PublicKeyID = v
	}

	// Parse time fields
	if exp, ok := c["exp"].(float64); ok {
		lic.ExpiresAt = time.Unix(int64(exp), 0)
	}
	if iat, ok := c["iat"].(float64); ok {
		lic.IssuedAt = time.Unix(int64(iat), 0)
	}

	// Parse features
	if features, ok := c["features"].([]interface{}); ok {
		for _, f := range features {
			if s, ok := f.(string); ok {
				lic.Features = append(lic.Features, s)
			}
		}
	}

	// Parse limits
	if limits, ok := c["limits"].(map[string]interface{}); ok {
		if n, ok := limits["max_nodes"].(float64); ok {
			lic.Limits.MaxNodes = int(n)
		}
		if n, ok := limits["max_clusters"].(float64); ok {
			lic.Limits.MaxClusters = int(n)
		}
	}

	return lic
}
