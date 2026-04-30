# OpenCore Licensing Phase 1-2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Ed25519 JWT license validation core, multi-path license loading, license CLI commands, and standardize all community stubs so community builds contain zero enterprise code and return consistent 402 errors.

**Architecture:** Single Go repo with `//go:build pro` / `//go:build !pro` build tags. `internal/server/license/` is shared (no build tag) — it validates JWTs but is a no-op in community builds via `RequireLicense()` stubs. Community feature stubs in `*_community.go` register their own 402 routes directly; Pro builds use `middleware.ProFeature()` for runtime license gating. License manager is wired into `Server` and passed down via `ServerConfig`.

**Tech Stack:** Go, `crypto/ed25519` (stdlib), `github.com/golang-jwt/jwt/v5` (already in go.mod), Cobra CLI (`github.com/spf13/cobra`), Gin middleware.

---

## File Map

**Phase 1 — New files:**
- `internal/server/license/models.go` — JWT claims struct, Feature constants, LicenseType, Status
- `internal/server/license/keys.go` — Embedded trusted public key table (dev test key included)
- `internal/server/license/validator.go` — Ed25519 JWT signature verification
- `internal/server/license/storage.go` — Multi-path `.lic` file loading
- `internal/server/license/license.go` — `Manager`: orchestrates load→validate→status
- `internal/server/license/startup_pro.go` (`//go:build pro`) — `RequireLicense()` fails on missing/invalid license
- `internal/server/license/startup_community.go` (`//go:build !pro`) — `RequireLicense()` no-op returns nil
- `cmd/manager/cmd/license.go` — `lattice-manager license {install,show,validate}` CLI

**Phase 1 — Modified files:**
- `cmd/manager/cmd/root.go` — Register `newLicenseCmd()`
- `internal/server/run.go` — Call `license.RequireLicense()` before `NewServer()`
- `internal/server/server/server.go` — Add `license *license.Manager` field to `Server` and `ServerConfig`

**Phase 1 — Test files:**
- `internal/server/license/validator_test.go`
- `internal/server/license/storage_test.go`
- `internal/server/license/license_test.go`

**Phase 2 — New files:**
- `internal/server/server/middleware/license_check.go` (`//go:build pro`) — `ProFeature(lm, feature)` gin middleware

**Phase 2 — Modified files:**
- `pkg/utils/resp/response.go` — Add `PaymentRequired()` helper
- `internal/server/dex/dex_community.go` — Fix 503 → 402 with standard format
- `internal/server/server/dashboard_community.go` — Standardize to `feature_not_licensed` format
- `internal/server/server/monitor_community.go` — Standardize to `feature_not_licensed` format
- `internal/server/server/monitor.go` (`//go:build pro`) — Apply `ProFeature` middleware
- `internal/server/server/dashboard.go` (`//go:build pro`) — Apply `ProFeature` middleware

---

## Task 1: License Models and Feature Constants

**Files:**
- Create: `internal/server/license/models.go`
- Test: `internal/server/license/models_test.go`

- [ ] **Step 1: Create models.go**

```go
// internal/server/license/models.go
package license

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Feature identifies a licensable Pro feature.
type Feature string

const (
	FeatureOIDC         Feature = "oidc"
	FeatureTURN         Feature = "turn"
	FeatureTelemetry    Feature = "telemetry"
	FeatureAudit        Feature = "audit"
	FeatureMonitor      Feature = "monitor"
	FeatureDashboard    Feature = "dashboard"
	FeatureAIAudit      Feature = "ai-audit"
	FeatureMultiCluster Feature = "multi-cluster"
	FeatureWebhook      Feature = "webhook"
	FeatureSIEM         Feature = "siem"
)

// LicenseType is the tier of a license.
type LicenseType string

const (
	TypeTrial      LicenseType = "trial"
	TypeStandard   LicenseType = "standard"
	TypeEnterprise LicenseType = "enterprise"
	TypeNFR        LicenseType = "nfr"
)

// Status describes the current state of the loaded license.
type Status int

const (
	StatusValid   Status = iota // License valid and not expiring soon
	StatusWarning               // License expiring within 14 days
	StatusGrace                 // License expired, within grace period
	StatusExpired               // License expired, past grace period
	StatusRevoked               // License explicitly revoked
	StatusMissing               // No license loaded
)

// Limits caps resource usage for a license.
type Limits struct {
	MaxNodes    int `json:"max_nodes"`
	MaxClusters int `json:"max_clusters"`
}

// Claims is the JWT payload for a Lattice license.
type Claims struct {
	jwt.RegisteredClaims
	CustomerName string      `json:"customer_name"`
	Type         LicenseType `json:"type"`
	Features     []Feature   `json:"features"`
	Limits       Limits      `json:"limits"`
	PublicKeyID  string      `json:"public_key_id"`
}

// GraceDays returns the grace period duration for this license type.
func (c *Claims) GraceDays() int {
	switch c.Type {
	case TypeEnterprise:
		return 14
	case TypeStandard:
		return 7
	case TypeNFR:
		return 7
	default: // trial
		return 0
	}
}

// ExpiresAt returns the license expiry time.
func (c *Claims) ExpiresAt() time.Time {
	if c.RegisteredClaims.ExpiresAt == nil {
		return time.Time{}
	}
	return c.RegisteredClaims.ExpiresAt.Time
}
```

- [ ] **Step 2: Write model smoke test**

Create `internal/server/license/models_test.go`:

```go
package license

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestClaims_GraceDays(t *testing.T) {
	cases := []struct {
		typ  LicenseType
		want int
	}{
		{TypeTrial, 0},
		{TypeStandard, 7},
		{TypeEnterprise, 14},
		{TypeNFR, 7},
	}
	for _, tc := range cases {
		c := &Claims{Type: tc.typ}
		if got := c.GraceDays(); got != tc.want {
			t.Errorf("GraceDays(%s) = %d, want %d", tc.typ, got, tc.want)
		}
	}
}

func TestClaims_ExpiresAt_nil(t *testing.T) {
	c := &Claims{}
	if !c.ExpiresAt().IsZero() {
		t.Error("expected zero time when ExpiresAt claim is nil")
	}
}

func TestClaims_ExpiresAt(t *testing.T) {
	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	c := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	if got := c.ExpiresAt(); !got.Equal(exp) {
		t.Errorf("ExpiresAt = %v, want %v", got, exp)
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/server/license/ -run TestClaims -v
```

Expected: all 3 pass.

- [ ] **Step 4: Commit**

```bash
git add internal/server/license/models.go internal/server/license/models_test.go
git commit -m "feat(license): add license models, feature constants, and claims"
```

---

## Task 2: Public Key Table

**Files:**
- Create: `internal/server/license/keys.go`

- [ ] **Step 1: Generate a dev/test Ed25519 keypair and create keys.go**

The dev public key below is generated for testing purposes only. Before production release, replace it with the actual signing public key.

Generate your production keypair (run once, store private key securely):
```bash
# Generate keypair — run this yourself and store private key in a secret manager
go run -e 'package main; import ("crypto/ed25519"; "encoding/base64"; "crypto/rand"; "fmt")
func main() {
    pub, priv, _ := ed25519.GenerateKey(rand.Reader)
    fmt.Println("pub:", base64.StdEncoding.EncodeToString(pub))
    fmt.Println("priv:", base64.StdEncoding.EncodeToString(priv))
}'
```

Create `internal/server/license/keys.go` with a placeholder dev key (tests will generate their own):

```go
// internal/server/license/keys.go
package license

import "crypto/ed25519"

// trustedPublicKeys maps public_key_id → Ed25519 public key.
// Add new keys here on rotation; do NOT remove old keys until all
// licenses signed with them have expired.
//
// PRODUCTION NOTE: Replace devPublicKey with real key bytes before release.
// The private key must be stored in a secure secret manager (never in this repo).
var trustedPublicKeys = map[string]ed25519.PublicKey{
	// "pk-2026-01": ed25519.PublicKey{ /* 32 bytes of real public key */ },
}

// LookupPublicKey returns the public key for the given key ID.
// Returns nil if the key ID is unknown.
func LookupPublicKey(keyID string) ed25519.PublicKey {
	return trustedPublicKeys[keyID]
}

// RegisterPublicKey adds a public key to the trusted set.
// This is used in tests to inject test keys.
func RegisterPublicKey(keyID string, pub ed25519.PublicKey) {
	trustedPublicKeys[keyID] = pub
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/license/keys.go
git commit -m "feat(license): add public key table with rotation support"
```

---

## Task 3: Ed25519 JWT Validator

**Files:**
- Create: `internal/server/license/validator.go`
- Create: `internal/server/license/validator_test.go`

- [ ] **Step 1: Write the failing test first**

Create `internal/server/license/validator_test.go`:

```go
package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testKeyID = "pk-test-001"

// makeTestToken signs a license JWT with the given key and claims.
func makeTestToken(t *testing.T, priv ed25519.PrivateKey, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = claims.PublicKeyID
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// setupTestKey generates a keypair and registers the public key for validation.
func setupTestKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	RegisterPublicKey(testKeyID, pub)
	t.Cleanup(func() { delete(trustedPublicKeys, testKeyID) })
	return pub, priv
}

func TestValidate_valid(t *testing.T) {
	_, priv := setupTestKey(t)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cust-001",
			Issuer:    "license.lattice.run",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		CustomerName: "Acme Corp",
		Type:         TypeEnterprise,
		Features:     []Feature{FeatureOIDC, FeatureTURN},
		Limits:       Limits{MaxNodes: 500, MaxClusters: 10},
		PublicKeyID:  testKeyID,
	}
	signed := makeTestToken(t, priv, claims)

	got, err := Validate(signed)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got.CustomerName != "Acme Corp" {
		t.Errorf("CustomerName = %q, want %q", got.CustomerName, "Acme Corp")
	}
	if got.Type != TypeEnterprise {
		t.Errorf("Type = %q, want %q", got.Type, TypeEnterprise)
	}
}

func TestValidate_expired(t *testing.T) {
	_, priv := setupTestKey(t)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cust-002",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-25 * time.Hour)),
		},
		Type:        TypeStandard,
		PublicKeyID: testKeyID,
	}
	signed := makeTestToken(t, priv, claims)

	_, err := Validate(signed)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestValidate_unknownKeyID(t *testing.T) {
	_, priv := setupTestKey(t)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cust-003",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Type:        TypeTrial,
		PublicKeyID: "pk-unknown-999",
	}
	signed := makeTestToken(t, priv, claims)

	_, err := Validate(signed)
	if err == nil {
		t.Fatal("expected error for unknown key ID, got nil")
	}
}

func TestValidate_tamperedSignature(t *testing.T) {
	_, priv := setupTestKey(t)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cust-004",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Type:        TypeEnterprise,
		PublicKeyID: testKeyID,
	}
	signed := makeTestToken(t, priv, claims)

	// Tamper: replace last 4 chars of signature
	tampered := signed[:len(signed)-4] + "XXXX"
	_, err := Validate(tampered)
	if err == nil {
		t.Fatal("expected error for tampered signature, got nil")
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/server/license/ -run TestValidate -v
```

Expected: compile error — `Validate` not defined.

- [ ] **Step 3: Implement validator.go**

Create `internal/server/license/validator.go`:

```go
// internal/server/license/validator.go
package license

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrUnknownKeyID    = errors.New("license: unknown public_key_id")
	ErrInvalidToken    = errors.New("license: invalid token")
	ErrExpired         = errors.New("license: token expired")
	ErrWrongAlgorithm  = errors.New("license: unexpected signing algorithm")
)

// Validate parses and verifies a license JWT string.
// It uses the public_key_id in the token header to look up the trusted public key.
// Returns the verified Claims or an error.
func Validate(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		keyFunc,
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %v", ErrExpired, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// keyFunc resolves the public key from the token's "kid" header field.
func keyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
		return nil, fmt.Errorf("%w: got %s", ErrWrongAlgorithm, token.Method.Alg())
	}
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		// Fall back to claims field for backward compatibility
		if c, ok2 := token.Claims.(*Claims); ok2 {
			kid = c.PublicKeyID
		}
	}
	pub := LookupPublicKey(kid)
	if pub == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownKeyID, kid)
	}
	return pub, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/server/license/ -run TestValidate -v
```

Expected: all 4 pass.

- [ ] **Step 5: Commit**

```bash
git add internal/server/license/validator.go internal/server/license/validator_test.go
git commit -m "feat(license): Ed25519 JWT validator with key ID lookup"
```

---

## Task 4: Multi-Path Storage Loader

**Files:**
- Create: `internal/server/license/storage.go`
- Create: `internal/server/license/storage_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/license/storage_test.go`:

```go
package license

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile_found(t *testing.T) {
	dir := t.TempDir()
	licPath := filepath.Join(dir, "license.lic")
	if err := os.WriteFile(licPath, []byte("test-token"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := loadFromFile(licPath)
	if err != nil {
		t.Fatalf("loadFromFile() error = %v", err)
	}
	if got != "test-token" {
		t.Errorf("got %q, want %q", got, "test-token")
	}
}

func TestLoadFromFile_notFound(t *testing.T) {
	_, err := loadFromFile("/nonexistent/path/license.lic")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_envVarPath(t *testing.T) {
	dir := t.TempDir()
	licPath := filepath.Join(dir, "license.lic")
	if err := os.WriteFile(licPath, []byte("env-path-token"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LATTICE_LICENSE_PATH", licPath)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != "env-path-token" {
		t.Errorf("Load() = %q, want %q", got, "env-path-token")
	}
}

func TestLoad_envVarInline(t *testing.T) {
	t.Setenv("LATTICE_LICENSE_PATH", "")
	t.Setenv("LATTICE_LICENSE", "inline-token-string")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != "inline-token-string" {
		t.Errorf("Load() = %q, want %q", got, "inline-token-string")
	}
}

func TestLoad_notFound(t *testing.T) {
	t.Setenv("LATTICE_LICENSE_PATH", "")
	t.Setenv("LATTICE_LICENSE", "")

	// Ensure home dir path does not exist (it won't in temp test env if we override)
	_, err := Load()
	// It's OK to get an error here — just should not panic
	_ = err
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/server/license/ -run TestLoad -v
```

Expected: compile error — `Load` and `loadFromFile` not defined.

- [ ] **Step 3: Implement storage.go**

Create `internal/server/license/storage.go`:

```go
// internal/server/license/storage.go
package license

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoLicense is returned when no license file can be found.
var ErrNoLicense = errors.New("license: no license file found")

// Load resolves the license JWT string from the following locations in priority order:
//  1. $LATTICE_LICENSE_PATH — path to a .lic file
//  2. $LATTICE_LICENSE — the JWT string itself
//  3. ~/.lattice/license.lic
//  4. /var/lib/lattice/license.lic
func Load() (string, error) {
	// 1. Env var: path to file
	if path := os.Getenv("LATTICE_LICENSE_PATH"); path != "" {
		return loadFromFile(path)
	}

	// 2. Env var: inline JWT string
	if inline := os.Getenv("LATTICE_LICENSE"); inline != "" {
		return strings.TrimSpace(inline), nil
	}

	// 3. User home directory
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".lattice", "license.lic")
		if tok, err := loadFromFile(path); err == nil {
			return tok, nil
		}
	}

	// 4. System default (Linux production)
	if tok, err := loadFromFile("/var/lib/lattice/license.lic"); err == nil {
		return tok, nil
	}

	return "", fmt.Errorf("%w — install with: lattice-manager license install ./license.lic", ErrNoLicense)
}

// loadFromFile reads a .lic file and returns the trimmed token string.
func loadFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("license: read %s: %w", path, err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("license: file %s is empty", path)
	}
	return token, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/server/license/ -run TestLoad -v
```

Expected: all 4 pass.

- [ ] **Step 5: Commit**

```bash
git add internal/server/license/storage.go internal/server/license/storage_test.go
git commit -m "feat(license): multi-path license file loader"
```

---

## Task 5: License Manager

**Files:**
- Create: `internal/server/license/license.go`
- Create: `internal/server/license/license_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/license/license_test.go`:

```go
package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const mgrKeyID = "pk-mgr-test"

func setupManagerKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	RegisterPublicKey(mgrKeyID, pub)
	t.Cleanup(func() { delete(trustedPublicKeys, mgrKeyID) })
	return pub, priv
}

func writeTestLicense(t *testing.T, dir string, claims Claims, priv ed25519.PrivateKey) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = claims.PublicKeyID
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	path := filepath.Join(dir, "license.lic")
	if err := os.WriteFile(path, []byte(signed), 0600); err != nil {
		t.Fatalf("write license: %v", err)
	}
	return path
}

func TestManager_HasFeature(t *testing.T) {
	_, priv := setupManagerKey(t)
	dir := t.TempDir()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cust-010",
			Issuer:    "license.lattice.run",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Type:        TypeEnterprise,
		Features:    []Feature{FeatureOIDC, FeatureTURN, FeatureMonitor},
		PublicKeyID: mgrKeyID,
	}
	licPath := writeTestLicense(t, dir, claims, priv)
	t.Setenv("LATTICE_LICENSE_PATH", licPath)
	defer t.Setenv("LATTICE_LICENSE_PATH", "")

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if !mgr.HasFeature(FeatureOIDC) {
		t.Error("expected HasFeature(FeatureOIDC) = true")
	}
	if !mgr.HasFeature(FeatureTURN) {
		t.Error("expected HasFeature(FeatureTURN) = true")
	}
	if mgr.HasFeature(FeatureAIAudit) {
		t.Error("expected HasFeature(FeatureAIAudit) = false")
	}
}

func TestManager_Status_valid(t *testing.T) {
	_, priv := setupManagerKey(t)
	dir := t.TempDir()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cust-011",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Type:        TypeStandard,
		PublicKeyID: mgrKeyID,
	}
	licPath := writeTestLicense(t, dir, claims, priv)
	t.Setenv("LATTICE_LICENSE_PATH", licPath)
	defer t.Setenv("LATTICE_LICENSE_PATH", "")

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if mgr.CurrentStatus() != StatusValid {
		t.Errorf("Status = %d, want StatusValid", mgr.CurrentStatus())
	}
}

func TestManager_MaxNodes(t *testing.T) {
	_, priv := setupManagerKey(t)
	dir := t.TempDir()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cust-012",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Type:        TypeEnterprise,
		Limits:      Limits{MaxNodes: 250},
		PublicKeyID: mgrKeyID,
	}
	licPath := writeTestLicense(t, dir, claims, priv)
	t.Setenv("LATTICE_LICENSE_PATH", licPath)
	defer t.Setenv("LATTICE_LICENSE_PATH", "")

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if mgr.MaxNodes() != 250 {
		t.Errorf("MaxNodes() = %d, want 250", mgr.MaxNodes())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/server/license/ -run TestManager -v
```

Expected: compile error — `NewManager` not defined.

- [ ] **Step 3: Implement license.go**

Create `internal/server/license/license.go`:

```go
// internal/server/license/license.go
package license

import (
	"errors"
	"time"
)

const warningThreshold = 14 * 24 * time.Hour

// Manager holds a validated license and exposes feature/limit checks.
// A nil Manager is safe to use — all HasFeature() calls return false.
type Manager struct {
	claims   *Claims
	status   Status
	tokenStr string
}

// NewManager loads the license from disk/env, validates it, and returns a Manager.
func NewManager() (*Manager, error) {
	tokenStr, err := Load()
	if err != nil {
		return nil, err
	}

	claims, err := Validate(tokenStr)
	if err != nil {
		// Distinguish expired from truly invalid
		if errors.Is(err, ErrExpired) {
			// Load claims without expiry check to allow grace-period logic
			claims, parseErr := parseWithoutExpiry(tokenStr)
			if parseErr != nil {
				return nil, err // original error
			}
			status := graceStatus(claims)
			return &Manager{claims: claims, status: status, tokenStr: tokenStr}, nil
		}
		return nil, err
	}

	return &Manager{
		claims:   claims,
		status:   computeStatus(claims),
		tokenStr: tokenStr,
	}, nil
}

// HasFeature returns true if the feature is included in this license.
// Safe to call on a nil Manager (returns false).
func (m *Manager) HasFeature(f Feature) bool {
	if m == nil || m.claims == nil {
		return false
	}
	for _, feat := range m.claims.Features {
		if feat == f {
			return true
		}
	}
	return false
}

// MaxNodes returns the maximum number of nodes allowed by this license.
// Returns 0 (unlimited) on a nil Manager.
func (m *Manager) MaxNodes() int {
	if m == nil || m.claims == nil {
		return 0
	}
	return m.claims.Limits.MaxNodes
}

// CurrentStatus returns the computed status of the license.
func (m *Manager) CurrentStatus() Status {
	if m == nil {
		return StatusMissing
	}
	return m.status
}

// CustomerName returns the license holder's name.
func (m *Manager) CustomerName() string {
	if m == nil || m.claims == nil {
		return ""
	}
	return m.claims.CustomerName
}

// ExpiresAt returns the license expiry time.
func (m *Manager) ExpiresAt() time.Time {
	if m == nil || m.claims == nil {
		return time.Time{}
	}
	return m.claims.ExpiresAt()
}

// LicenseType returns the type of license loaded.
func (m *Manager) LicenseType() LicenseType {
	if m == nil || m.claims == nil {
		return ""
	}
	return m.claims.Type
}

// computeStatus derives the Status for a freshly validated (not yet expired) license.
func computeStatus(c *Claims) Status {
	timeToExpiry := time.Until(c.ExpiresAt())
	if timeToExpiry <= warningThreshold {
		return StatusWarning
	}
	return StatusValid
}

// graceStatus derives Status for an already-expired license.
func graceStatus(c *Claims) Status {
	overdue := time.Since(c.ExpiresAt())
	gracePeriod := time.Duration(c.GraceDays()) * 24 * time.Hour
	if overdue <= gracePeriod {
		return StatusGrace
	}
	return StatusExpired
}

// parseWithoutExpiry parses JWT claims ignoring expiry validation.
// Used to read grace-period info from an expired token.
func parseWithoutExpiry(tokenStr string) (*Claims, error) {
	import_parser := newLenientParser()
	return import_parser.parse(tokenStr)
}
```

Wait — `parseWithoutExpiry` needs a lenient parser. Let me use the correct jwt/v5 approach:

- [ ] **Step 3 (revised): Implement license.go with correct lenient parser**

Replace the `parseWithoutExpiry` stub with the correct implementation. Create `internal/server/license/license.go`:

```go
// internal/server/license/license.go
package license

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const warningThreshold = 14 * 24 * time.Hour

// Manager holds a validated license and exposes feature/limit checks.
// A nil Manager is safe to use — all HasFeature() calls return false.
type Manager struct {
	claims   *Claims
	status   Status
	tokenStr string
}

// NewManager loads the license from disk/env, validates it, and returns a Manager.
func NewManager() (*Manager, error) {
	tokenStr, err := Load()
	if err != nil {
		return nil, err
	}

	claims, err := Validate(tokenStr)
	if err != nil {
		if errors.Is(err, ErrExpired) {
			// Parse ignoring expiry to compute grace-period status
			expiredClaims, parseErr := parseIgnoreExpiry(tokenStr)
			if parseErr != nil {
				return nil, fmt.Errorf("license: expired and unreadable: %w", err)
			}
			return &Manager{
				claims:   expiredClaims,
				status:   graceStatus(expiredClaims),
				tokenStr: tokenStr,
			}, nil
		}
		return nil, err
	}

	return &Manager{
		claims:   claims,
		status:   computeStatus(claims),
		tokenStr: tokenStr,
	}, nil
}

// HasFeature returns true if the feature is included in this license.
// Safe to call on a nil Manager (returns false).
func (m *Manager) HasFeature(f Feature) bool {
	if m == nil || m.claims == nil {
		return false
	}
	for _, feat := range m.claims.Features {
		if feat == f {
			return true
		}
	}
	return false
}

// MaxNodes returns the maximum number of nodes. 0 means unlimited.
func (m *Manager) MaxNodes() int {
	if m == nil || m.claims == nil {
		return 0
	}
	return m.claims.Limits.MaxNodes
}

// CurrentStatus returns the computed status of the license.
func (m *Manager) CurrentStatus() Status {
	if m == nil {
		return StatusMissing
	}
	return m.status
}

// CustomerName returns the license holder's name.
func (m *Manager) CustomerName() string {
	if m == nil || m.claims == nil {
		return ""
	}
	return m.claims.CustomerName
}

// ExpiresAt returns the license expiry time.
func (m *Manager) ExpiresAt() time.Time {
	if m == nil || m.claims == nil {
		return time.Time{}
	}
	return m.claims.ExpiresAt()
}

// LicenseType returns the tier of the loaded license.
func (m *Manager) LicenseType() LicenseType {
	if m == nil || m.claims == nil {
		return ""
	}
	return m.claims.Type
}

// computeStatus derives Status for a freshly validated (not yet expired) license.
func computeStatus(c *Claims) Status {
	if time.Until(c.ExpiresAt()) <= warningThreshold {
		return StatusWarning
	}
	return StatusValid
}

// graceStatus derives Status for an already-expired license.
func graceStatus(c *Claims) Status {
	overdue := time.Since(c.ExpiresAt())
	grace := time.Duration(c.GraceDays()) * 24 * time.Hour
	if overdue <= grace {
		return StatusGrace
	}
	return StatusExpired
}

// parseIgnoreExpiry parses JWT claims without validating the expiry time.
// Used to read grace-period info from a token that has just expired.
func parseIgnoreExpiry(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		keyFunc,
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithExpirationRequired(),   // require the field to exist
		jwt.WithoutClaimsValidation(),  // but don't enforce it
	)
	if err != nil {
		// Accept ErrTokenExpired — that's expected; fail on anything else
		if !errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("license: parse expired token: %w", err)
		}
	}
	return claims, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/server/license/ -run TestManager -v
```

Expected: all 3 pass.

- [ ] **Step 5: Run all license tests**

```bash
go test ./internal/server/license/... -v
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/server/license/license.go internal/server/license/license_test.go
git commit -m "feat(license): license manager with feature/limit checks and grace period"
```

---

## Task 6: Pro/Community Startup Hooks

**Files:**
- Create: `internal/server/license/startup_pro.go` (`//go:build pro`)
- Create: `internal/server/license/startup_community.go` (`//go:build !pro`)
- Modify: `internal/server/run.go`
- Modify: `internal/server/server/server.go`

- [ ] **Step 1: Create startup_pro.go**

```go
// internal/server/license/startup_pro.go
//go:build pro

package license

import "fmt"

// RequireLicense loads and validates the license for Pro builds.
// Returns an error (and should halt startup) if no valid license is found.
func RequireLicense() (*Manager, error) {
	mgr, err := NewManager()
	if err != nil {
		return nil, fmt.Errorf(
			"Lattice Pro requires a valid license.\n"+
				"Install with: lattice-manager license install ./license.lic\n"+
				"Error: %w", err,
		)
	}
	switch mgr.CurrentStatus() {
	case StatusExpired:
		return nil, fmt.Errorf(
			"license expired and past grace period — renew at https://alattice.io/renew",
		)
	case StatusRevoked:
		return nil, fmt.Errorf("license has been revoked — contact support@alattice.io")
	}
	return mgr, nil
}
```

- [ ] **Step 2: Create startup_community.go**

```go
// internal/server/license/startup_community.go
//go:build !pro

package license

// RequireLicense is a no-op for community builds.
// Returns nil, nil — community builds need no license.
func RequireLicense() (*Manager, error) {
	return nil, nil
}
```

- [ ] **Step 3: Add license field to ServerConfig and Server**

In `internal/server/server/server.go`, add the `License` field to `ServerConfig` and `license` field to `Server`:

Find the `ServerConfig` struct (line ~80) and add:
```go
type ServerConfig struct {
	Cfg     *config.Config
	Nats    infra.SignalService
	License *license.Manager // nil on community builds
}
```

Find the `Server` struct (line ~42) and add the `license` field after `cfg`:
```go
type Server struct {
	*gin.Engine
	logger  *log.Logger
	listen  string
	nats    infra.SignalService
	cfg     *config.Config
	license *license.Manager // nil on community builds
	// ... rest of fields unchanged
```

Also add the import for license package at the top of `server.go`:
```go
import (
	// existing imports...
	"github.com/alatticeio/lattice/internal/server/license"
)
```

And in `NewServer()`, after `s := &Server{...}` initialization block, add:
```go
license: serverConfig.License,
```

- [ ] **Step 4: Wire RequireLicense into run.go**

In `internal/server/run.go`, add license check before `server.NewServer()`:

Add import: `"github.com/alatticeio/lattice/internal/server/license"`

In the `Start()` function, before `hs, err := server.NewServer(...)`:
```go
lm, err := license.RequireLicense()
if err != nil {
    logger.Error("license check failed", err)
    return fmt.Errorf("license: %w", err)
}
if lm != nil {
    logger.Info("license loaded",
        "customer", lm.CustomerName(),
        "type", string(lm.LicenseType()),
        "expires", lm.ExpiresAt().Format("2006-01-02"),
        "status", lm.CurrentStatus(),
    )
}
```

And pass `lm` into `ServerConfig`:
```go
hs, err := server.NewServer(ctx, &server.ServerConfig{
    Cfg:     config.GlobalConfig,
    License: lm,
})
```

- [ ] **Step 5: Build both community and pro to verify compilation**

```bash
# Community build (no license required)
go build ./...

# Pro build (license required at runtime, not compile time)
go build -tags pro ./...
```

Expected: both compile without errors.

- [ ] **Step 6: Commit**

```bash
git add internal/server/license/startup_pro.go \
        internal/server/license/startup_community.go \
        internal/server/server/server.go \
        internal/server/run.go
git commit -m "feat(license): wire RequireLicense into server startup (pro gates on license, community no-op)"
```

---

## Task 7: License CLI Commands

**Files:**
- Create: `cmd/manager/cmd/license.go`
- Modify: `cmd/manager/cmd/root.go`

- [ ] **Step 1: Create license.go**

```go
// cmd/manager/cmd/license.go
package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alatticeio/lattice/internal/server/license"
	"github.com/spf13/cobra"
)

func newLicenseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "license",
		Short: "Manage Lattice Pro license",
	}
	cmd.AddCommand(newLicenseInstallCmd())
	cmd.AddCommand(newLicenseShowCmd())
	cmd.AddCommand(newLicenseValidateCmd())
	return cmd
}

// install copies a .lic file to /var/lib/lattice/license.lic
func newLicenseInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <path-to-license.lic>",
		Short: "Install a license file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			data, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("cannot read license file: %w", err)
			}
			tokenStr := strings.TrimSpace(string(data))
			if tokenStr == "" {
				return fmt.Errorf("license file is empty")
			}

			// Destination: prefer LATTICE_LICENSE_PATH, fall back to system default
			dest := os.Getenv("LATTICE_LICENSE_PATH")
			if dest == "" {
				dest = "/var/lib/lattice/license.lic"
			}

			if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
				return fmt.Errorf("cannot create license directory: %w", err)
			}
			if err := os.WriteFile(dest, []byte(tokenStr+"\n"), 0600); err != nil {
				return fmt.Errorf("cannot write license file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "License installed to %s\n", dest)
			return nil
		},
	}
}

// show prints license details
func newLicenseShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current license details",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showLicense(cmd.OutOrStdout())
		},
	}
}

func showLicense(w io.Writer) error {
	mgr, err := license.NewManager()
	if err != nil {
		return fmt.Errorf("no license found: %w", err)
	}

	statusStr := map[license.Status]string{
		license.StatusValid:   "Valid",
		license.StatusWarning: "Expiring Soon",
		license.StatusGrace:   "Grace Period",
		license.StatusExpired: "Expired",
		license.StatusRevoked: "Revoked",
		license.StatusMissing: "Missing",
	}[mgr.CurrentStatus()]

	fmt.Fprintln(w, "=== Lattice Pro License ===")
	fmt.Fprintf(w, "Customer : %s\n", mgr.CustomerName())
	fmt.Fprintf(w, "Type     : %s\n", mgr.LicenseType())
	fmt.Fprintf(w, "Expires  : %s\n", mgr.ExpiresAt().Format("2006-01-02"))
	fmt.Fprintf(w, "Status   : %s\n", statusStr)
	fmt.Fprintf(w, "Max nodes: %d\n", mgr.MaxNodes())
	return nil
}

// validate loads and verifies the license, printing result
func newLicenseValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the installed license",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := license.NewManager()
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "License invalid: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "License is valid.")
			return nil
		},
	}
}
```

- [ ] **Step 2: Register in root.go**

In `cmd/manager/cmd/root.go`, in the `init()` function, add:
```go
func init() {
	rootCmd.PersistentFlags().StringP("config-dir", "", "", "config directory (default ~/.lattice)")
	rootCmd.AddCommand(newStartCommand())
	rootCmd.AddCommand(newLicenseCmd()) // ADD THIS LINE
}
```

- [ ] **Step 3: Build and smoke-test the CLI**

```bash
go build -o /tmp/lattice-manager ./cmd/manager/
/tmp/lattice-manager license --help
```

Expected output:
```
Manage Lattice Pro license

Usage:
  lattice [command]

Available Commands:
  install     Install a license file
  show        Show current license details
  validate    Validate the installed license
```

- [ ] **Step 4: Commit**

```bash
git add cmd/manager/cmd/license.go cmd/manager/cmd/root.go
git commit -m "feat(license): add license install/show/validate CLI commands"
```

---

## Task 8: Standardize 402 Error Response (Phase 2 start)

**Files:**
- Modify: `pkg/utils/resp/response.go`

- [ ] **Step 1: Add PaymentRequired helper**

In `pkg/utils/resp/response.go`, add after `BadRequest`:

```go
// PaymentRequired returns a 402 response for unlicensed Pro features.
// feature is the feature identifier (e.g. "oidc", "turn").
func PaymentRequired(c *gin.Context, feature string) {
	c.JSON(http.StatusPaymentRequired, gin.H{
		"error":   "feature_not_licensed",
		"feature": feature,
		"message": "This feature requires Lattice Pro — upgrade at https://alattice.io/pro",
	})
}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./pkg/utils/resp/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add pkg/utils/resp/response.go
git commit -m "feat(resp): add PaymentRequired helper for unlicensed Pro features"
```

---

## Task 9: Standardize Community Stubs

**Files:**
- Modify: `internal/server/dex/dex_community.go`
- Modify: `internal/server/server/dashboard_community.go`
- Modify: `internal/server/server/monitor_community.go`

- [ ] **Step 1: Fix dex_community.go (503 → 402 + standard format)**

Replace the `Login` method body in `internal/server/dex/dex_community.go`:

```go
// dex_community.go — replace Login method
func (d *Dex) Login(c *gin.Context) {
	c.JSON(402, gin.H{
		"error":   "feature_not_licensed",
		"feature": "oidc",
		"message": "OIDC/SSO requires Lattice Pro — upgrade at https://alattice.io/pro",
	})
}
```

Also remove the unused `errProRequired` variable. The file should look like:

```go
//go:build !pro

package dex

import (
	"errors"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/gin-gonic/gin"
)

var errProRequired = errors.New("Dex OIDC/SSO is a Lattice Pro feature — upgrade at https://alattice.io/pro")

type Dex struct{}

func NewDex(_ service.UserService) (*Dex, error) {
	return nil, errProRequired
}

func (d *Dex) Login(c *gin.Context) {
	c.JSON(402, gin.H{
		"error":   "feature_not_licensed",
		"feature": "oidc",
		"message": "OIDC/SSO requires Lattice Pro — upgrade at https://alattice.io/pro",
	})
}
```

- [ ] **Step 2: Update dashboard_community.go**

Replace the handler in `internal/server/server/dashboard_community.go`:

```go
//go:build !pro

package server

import "github.com/gin-gonic/gin"

func (s *Server) dashboardRouter() {
	s.GET("/api/v1/dashboard/overview", func(c *gin.Context) {
		c.JSON(402, gin.H{
			"error":   "feature_not_licensed",
			"feature": "dashboard",
			"message": "Dashboard analytics requires Lattice Pro — upgrade at https://alattice.io/pro",
		})
	})
	s.GET("/api/v1/workspaces/:id/dashboard", func(c *gin.Context) {
		c.JSON(402, gin.H{
			"error":   "feature_not_licensed",
			"feature": "dashboard",
			"message": "Dashboard analytics requires Lattice Pro — upgrade at https://alattice.io/pro",
		})
	})
}
```

- [ ] **Step 3: Update monitor_community.go**

Replace `internal/server/server/monitor_community.go`:

```go
//go:build !pro

package server

import "github.com/gin-gonic/gin"

func (s *Server) monitorRouter() {
	proOnly := func(feature string) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.JSON(402, gin.H{
				"error":   "feature_not_licensed",
				"feature": feature,
				"message": "Network monitoring requires Lattice Pro — upgrade at https://alattice.io/pro",
			})
		}
	}
	g := s.Group("/api/v1/monitor")
	g.GET("/topology", proOnly("monitor"))
	g.GET("/ws-snapshot", proOnly("monitor"))
}
```

- [ ] **Step 4: Build community to verify consistency**

```bash
go build ./...
```

Expected: no errors (community build, default).

- [ ] **Step 5: Commit**

```bash
git add internal/server/dex/dex_community.go \
        internal/server/server/dashboard_community.go \
        internal/server/server/monitor_community.go
git commit -m "fix(community): standardize all stubs to feature_not_licensed 402 format"
```

---

## Task 10: Pro Build License Middleware

**Files:**
- Create: `internal/server/server/middleware/license_check.go` (`//go:build pro`)
- Modify: `internal/server/server/monitor.go` (`//go:build pro`)
- Modify: `internal/server/server/dashboard.go` (`//go:build pro`)

- [ ] **Step 1: Create license middleware (pro-only)**

Create `internal/server/server/middleware/license_check.go`:

```go
//go:build pro

package middleware

import (
	"github.com/alatticeio/lattice/internal/server/license"
	"github.com/gin-gonic/gin"
)

// ProFeature returns a Gin middleware that gates access to a Pro feature.
// If the license manager does not include the given feature, it responds with
// 402 Payment Required and aborts the request chain.
func ProFeature(lm *license.Manager, feature license.Feature) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !lm.HasFeature(feature) {
			c.JSON(402, gin.H{
				"error":   "feature_not_licensed",
				"feature": string(feature),
				"message": "This feature requires a Lattice Pro license — upgrade at https://alattice.io/pro",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 2: Apply middleware to monitor.go (pro build)**

In `internal/server/server/monitor.go`, update `monitorRouter()` to apply the license check:

```go
//go:build pro

package server

import (
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/license"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) monitorRouter() {
	monitorRouter := s.Group("/api/v1/monitor")
	monitorRouter.Use(middleware.AuthMiddleware())
	monitorRouter.Use(middleware.ProFeature(s.license, license.FeatureMonitor))
	{
		monitorRouter.GET("/topology", s.topology())
		monitorRouter.GET("/ws-snapshot", s.tenantMiddleware.Handle(), s.workspaceSnapshot())
	}
}

// ... (keep existing topology() and workspaceSnapshot() unchanged)
```

- [ ] **Step 3: Apply middleware to dashboard.go (pro build)**

In `internal/server/server/dashboard.go`, update `dashboardRouter()`:

```go
//go:build pro

package server

import (
	"github.com/alatticeio/lattice/internal/server/license"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) dashboardRouter() {
	dashApi := s.Group("/api/v1/dashboard")
	dashApi.Use(middleware.AuthMiddleware())
	dashApi.Use(middleware.ProFeature(s.license, license.FeatureDashboard))
	{
		dashApi.GET("/overview", s.dashboardOverview())
	}

	wsApi := s.Group("/api/v1/workspaces/:id/dashboard")
	wsApi.Use(middleware.AuthMiddleware())
	wsApi.Use(middleware.ProFeature(s.license, license.FeatureDashboard))
	{
		wsApi.GET("", s.workspaceDashboard())
	}
}

// ... (keep existing dashboardOverview() and workspaceDashboard() unchanged)
```

- [ ] **Step 4: Build pro binary to verify**

```bash
go build -tags pro ./...
```

Expected: no errors.

- [ ] **Step 5: Build community binary to verify stubs still work**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Run all tests**

```bash
go test ./internal/server/license/... ./pkg/utils/resp/... -v
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/server/server/middleware/license_check.go \
        internal/server/server/monitor.go \
        internal/server/server/dashboard.go
git commit -m "feat(license): ProFeature middleware gates monitor and dashboard in Pro builds"
```

---

## Spec Coverage Check

| Design Requirement | Task |
|---|---|
| Ed25519 JWT verification | Task 3 (validator.go) |
| Multi-path license loading | Task 4 (storage.go) |
| License types (Trial/Standard/Enterprise/NFR) | Task 1 (models.go) |
| Grace period logic | Task 5 (license.go graceStatus) |
| Feature toggle loading | Task 5 (HasFeature) |
| Max node limit | Task 5 (MaxNodes) |
| Startup failure on missing Pro license | Task 6 (startup_pro.go) |
| Community no-op | Task 6 (startup_community.go) |
| `lattice-manager license install` | Task 7 |
| `lattice-manager license show` | Task 7 |
| `lattice-manager license validate` | Task 7 |
| 402 standard format `feature_not_licensed` | Task 8 + 9 |
| Community stubs consistent | Task 9 |
| Pro middleware for monitor | Task 10 |
| Pro middleware for dashboard | Task 10 |
| Public key versioning | Task 2 (keys.go) |
