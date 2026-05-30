# Agent Binary Size Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce `lattice` agent binary (Linux production) from ~37MB to ~20MB by cleaning up server-side dependency pollution and adding `-trimpath`.

**Architecture:** Five targeted changes — delete unused redis helper, relocate server-only jwt/validator helpers to `internal/server/utils`, fix a single gorm import in `server/vo/label.go`, replace `server/vo` types in `agent/client` with lightweight local structs, and add `-trimpath` to Makefile. No behavioral changes; all agent features remain intact.

**Tech Stack:** Go 1.25, golangci-lint, standard library

---

## File Map

| Action | File |
|--------|------|
| Delete | `pkg/utils/redis.go` |
| Delete | `pkg/utils/jwt_test.go` |
| Modify | `pkg/utils/validator.go` → move to `internal/server/utils/validator.go` |
| Modify | `pkg/utils/jwt.go` → move to `internal/server/utils/jwt.go` |
| Create | `internal/server/utils/jwt_test.go` |
| Modify | `internal/server/vo/label.go` |
| Modify | `internal/agent/client/admin.go` |
| Modify | `internal/server/server/server.go` |
| Modify | `internal/server/server/user.go` |
| Modify | `internal/server/server/demo.go` |
| Modify | `internal/server/server/middleware/auth.go` |
| Modify | `internal/server/server/middleware/permission.go` |
| Modify | `internal/server/dex/auth_middleware.go` |
| Modify | `internal/server/dex/login.go` |
| Modify | `internal/server/service/auth.go` |
| Modify | `internal/server/service/invitation.go` |
| Modify | `Makefile` |

---

## Task 1: Add -trimpath to Makefile

**Files:**
- Modify: `Makefile:104`

- [ ] **Step 1: Add -trimpath to the main build target**

In `Makefile`, line 104, change:
```makefile
		-ldflags="-s -w $(LDFLAGS)" \
```
to:
```makefile
		-trimpath -ldflags="-s -w $(LDFLAGS)" \
```

Also update line 77 (build-mcp target) the same way:
```makefile
		-trimpath -ldflags="-s -w $(LDFLAGS)" \
```

- [ ] **Step 2: Verify build still works**

```bash
make build SERVICE=lattice
```
Expected: `✅ Built: bin/lattice` — binary size slightly smaller than before.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -s -m "build: add -trimpath to reduce binary size and strip build paths"
```

---

## Task 2: Delete pkg/utils/redis.go

`RedisClient` has zero callers anywhere in the codebase. Deleting it removes `github.com/redis/go-redis/v9` from `pkg/utils`.

**Files:**
- Delete: `pkg/utils/redis.go`

- [ ] **Step 1: Verify no callers exist**

```bash
grep -rn "utils\.NewClient\b\|RedisClient\b" --include="*.go" .
```
Expected: no output (or only test files if any).

- [ ] **Step 2: Delete the file**

```bash
rm pkg/utils/redis.go
```

- [ ] **Step 3: Verify build passes**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 4: Verify lint passes**

```bash
make lint
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add -u pkg/utils/redis.go
git commit -s -m "refactor: remove unused RedisClient from pkg/utils"
```

---

## Task 3: Move pkg/utils/validator.go to internal/server/utils

The `init()` in `validator.go` registers custom gin binding validators (`safe_string`, `cidr`). Moving it to a server-only package removes `gin/binding` from the `pkg/utils` import graph.

**Files:**
- Create: `internal/server/utils/validator.go`
- Delete: `pkg/utils/validator.go`

- [ ] **Step 1: Create internal/server/utils/validator.go**

```go
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

package serverutils

import (
	"net"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("safe_string", func(fl validator.FieldLevel) bool {
			return IsSafeString(fl.Field().String())
		}); err != nil {
			panic("failed to register safe_string validator: " + err.Error())
		}
		if err := v.RegisterValidation("cidr", func(fl validator.FieldLevel) bool {
			return IsValidCIDR(fl.Field().String())
		}); err != nil {
			panic("failed to register cidr validator: " + err.Error())
		}
	}
}

var safeStringRE = regexp.MustCompile(`^[a-zA-Z0-9_\-\. ]+$`)

// IsSafeString returns true if s contains no HTML/JS injection characters.
func IsSafeString(s string) bool {
	return safeStringRE.MatchString(s)
}

// IsValidCIDR returns true if s is a valid CIDR notation.
func IsValidCIDR(s string) bool {
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

// SanitizeString trims and removes null bytes.
func SanitizeString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}
```

- [ ] **Step 2: Add a blank import in server entry point to trigger init()**

The `init()` in `validator.go` must run before gin handles requests. Add a blank import in `internal/server/server/server.go`:

In the import block of `internal/server/server/server.go`, add:
```go
_ "github.com/alatticeio/lattice/internal/server/utils"
```

- [ ] **Step 3: Delete pkg/utils/validator.go**

```bash
rm pkg/utils/validator.go
```

- [ ] **Step 4: Verify build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 5: Verify lint**

```bash
make lint
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/server/utils/validator.go internal/server/server/server.go
git add -u pkg/utils/validator.go
git commit -s -m "refactor: move gin validator init from pkg/utils to internal/server/utils"
```

---

## Task 4: Move pkg/utils/jwt.go to internal/server/utils

`jwt.go` imports `server/models.LatticeClaims` which pulls gorm into `pkg/utils`. Moving it to `internal/server/utils` breaks the `agent → pkg/utils → server/models → gorm` chain.

**Files:**
- Create: `internal/server/utils/jwt.go`
- Create: `internal/server/utils/jwt_test.go`
- Delete: `pkg/utils/jwt.go`
- Delete: `pkg/utils/jwt_test.go`
- Modify: `internal/server/server/server.go`
- Modify: `internal/server/server/user.go`
- Modify: `internal/server/server/demo.go`
- Modify: `internal/server/server/middleware/auth.go`
- Modify: `internal/server/server/middleware/permission.go`
- Modify: `internal/server/dex/auth_middleware.go`
- Modify: `internal/server/dex/login.go`
- Modify: `internal/server/service/auth.go`
- Modify: `internal/server/service/invitation.go`

- [ ] **Step 1: Create internal/server/utils/jwt.go**

```go
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

package serverutils

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/alatticeio/lattice/internal/server/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func ParseToken(tokenString string) (*models.LatticeClaims, error) {
	claims := &models.LatticeClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing algorithm: %v", token.Header["alg"])
		}
		return GetJWTSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if token.Valid {
		return claims, nil
	}
	return nil, errors.New("token verification failed: invalid credentials")
}

func GetJWTSecret() []byte {
	secret := os.Getenv("LATTICE_JWT_SECRET")
	if secret == "" {
		return []byte("your-256-bit-secret-key-here")
	}
	return []byte(secret)
}

// GenerateBusinessJWT issues a short-lived JWT (12h) for Dashboard sessions.
func GenerateBusinessJWT(userID, email, username, systemRole string) (string, error) {
	return GenerateBusinessJWTWithDuration(userID, email, username, systemRole, 12*time.Hour)
}

// GenerateBusinessJWTWithDuration issues a JWT with an explicit lifetime.
func GenerateBusinessJWTWithDuration(userID, email, username, systemRole string, duration time.Duration) (string, error) {
	claims := models.LatticeClaims{
		Email:      email,
		Username:   username,
		SystemRole: systemRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "lattice-bff",
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}
```

- [ ] **Step 2: Create internal/server/utils/jwt_test.go**

```go
package serverutils

import (
	"testing"

	"github.com/alatticeio/lattice/internal/server/models"
)

func TestGetJWTSecret(t *testing.T) {
	t.Run("should generate and parse JWT with jti", func(t *testing.T) {
		user := models.User{
			Email: "admin@123.com",
		}
		user.ID = "123"

		businessToken, err := GenerateBusinessJWT(user.ID, user.Email, user.Username, "")
		if err != nil {
			t.Fatal(err)
		}

		claims, err := ParseToken(businessToken)
		if err != nil {
			t.Fatal(err)
		}

		if claims.ID == "" {
			t.Error("expected jti (ID) to be non-empty")
		}
		if claims.Subject != "123" {
			t.Errorf("expected sub=123, got %s", claims.Subject)
		}
	})
}
```

- [ ] **Step 3: Run the new test**

```bash
go test ./internal/server/utils/...
```
Expected: PASS

- [ ] **Step 4: Delete the old files**

```bash
rm pkg/utils/jwt.go pkg/utils/jwt_test.go
```

- [ ] **Step 5: Update all callers — replace import path in 9 server files**

For each file below, replace the import `"github.com/alatticeio/lattice/pkg/utils"` usage of jwt functions:
- Any call to `utils.ParseToken(...)` → `serverutils.ParseToken(...)`
- Any call to `utils.GetJWTSecret()` → `serverutils.GetJWTSecret()`
- Any call to `utils.GenerateBusinessJWT(...)` → `serverutils.GenerateBusinessJWT(...)`
- Any call to `utils.GenerateBusinessJWTWithDuration(...)` → `serverutils.GenerateBusinessJWTWithDuration(...)`

Add `serverutils "github.com/alatticeio/lattice/internal/server/utils"` to the import block of each file that used these jwt functions. If the file no longer uses any other `utils.*` function after removing jwt calls, remove the `pkg/utils` import entirely.

Files to update:
1. `internal/server/server/server.go` — `utils.GetJWTSecret()` at line 308
2. `internal/server/server/user.go` — `utils.GenerateBusinessJWTWithDuration`
3. `internal/server/server/demo.go` — `utils.GenerateBusinessJWTWithDuration`
4. `internal/server/server/middleware/auth.go` — `utils.ParseToken`
5. `internal/server/server/middleware/permission.go` — `utils.ParseToken`
6. `internal/server/dex/auth_middleware.go` — `utils.GetJWTSecret()`
7. `internal/server/dex/login.go` — `utils.GenerateBusinessJWT`
8. `internal/server/service/auth.go` — `utils.GenerateBusinessJWTWithDuration`
9. `internal/server/service/invitation.go` — `utils.GenerateBusinessJWT`

- [ ] **Step 6: Verify build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 7: Run tests**

```bash
make test
```
Expected: all pass.

- [ ] **Step 8: Lint**

```bash
make lint
```
Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add internal/server/utils/jwt.go internal/server/utils/jwt_test.go
git add -u pkg/utils/jwt.go pkg/utils/jwt_test.go
git add internal/server/server/server.go internal/server/server/user.go internal/server/server/demo.go
git add internal/server/server/middleware/auth.go internal/server/server/middleware/permission.go
git add internal/server/dex/auth_middleware.go internal/server/dex/login.go
git add internal/server/service/auth.go internal/server/service/invitation.go
git commit -s -m "refactor: move JWT helpers from pkg/utils to internal/server/utils"
```

---

## Task 5: Remove gorm.DeletedAt from server/vo/label.go

`server/vo/label.go` is the sole reason `server/vo` imports `gorm.io/gorm`. Replacing `gorm.DeletedAt` with `*time.Time` removes gorm from the entire `server/vo` package.

**Files:**
- Modify: `internal/server/vo/label.go`

- [ ] **Step 1: Verify LabelVo.DeletedAt callers don't use gorm-specific methods**

```bash
grep -rn "\.DeletedAt\b" --include="*.go" internal/server/vo/ internal/server/service/ internal/server/controller/ internal/db/gormstore/
```

Check: any code that accesses `LabelVo.DeletedAt.Valid` or `LabelVo.DeletedAt.Time` must be updated. The `gorm.DeletedAt` struct has `.Valid bool` and `.Time time.Time`; a plain `*time.Time` represents nil as not-deleted and non-nil as deleted.

- [ ] **Step 2: Update internal/server/vo/label.go**

Replace the file content:
```go
package vo

import "time"

type LabelVo struct {
	ID          uint64     `json:"id"`
	Label       string     `json:"name"`
	CreatedAt   time.Time  `json:"createdAt"`
	DeletedAt   *time.Time `json:"deletedAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CreatedBy   string     `json:"createdBy"`
	UpdatedBy   string     `json:"updatedBy"`
	Description string     `json:"description"`
}

// NodeLabelVo Peer label relation
type NodeLabelVo struct {
	ModelVo
	NodeId    uint64
	LabelId   uint64
	LabelName string
	CreatedBy string
	UpdatedBy string
}
```

- [ ] **Step 3: Fix any callers that used .Valid / .Time**

If the grep in Step 1 found callers using `LabelVo.DeletedAt.Valid`, update them:
- `x.DeletedAt.Valid` → `x.DeletedAt != nil`
- `x.DeletedAt.Time` → `*x.DeletedAt`

- [ ] **Step 4: Build and lint**

```bash
go build ./...
make lint
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/server/vo/label.go
git commit -s -m "refactor: replace gorm.DeletedAt with *time.Time in LabelVo"
```

---

## Task 6: Replace server/vo types in agent/client with local view structs

`internal/agent/client/admin.go` imports `server/vo` for three types: `WorkspaceVo`, `PolicyVo`, `TokenVo`. `PolicyVo` embeds `*v1alpha1.LatticePolicySpec` (k8s CRD) and `TokenVo` uses `metav1.Time` — both pulling in `k8s.io/apimachinery`. Replacing them with local structs using only the JSON fields the CLI actually renders removes k8s entirely from the agent.

**Files:**
- Modify: `internal/agent/client/admin.go`

- [ ] **Step 1: Verify which fields are actually used in admin.go**

Check admin.go uses these fields:
- `WorkspaceVo`: `ID`, `Namespace`, `Slug`, `DisplayName`, `NodeCount`, `Status`
- `PolicyVo`: `Name`, `Action`, `PolicyTypes`, `Description`
- `TokenVo`: `Token`, `Namespace`, `UsageLimit`, `Expiry` (used as `.IsZero()` and `.Format(...)`)

- [ ] **Step 2: Replace server/vo import with local view types in admin.go**

At the top of `internal/agent/client/admin.go`, replace:
```go
import (
    "context"
    "fmt"
    "net/http"
    "os"
    "strings"
    "text/tabwriter"

    "github.com/alatticeio/lattice/internal/server/vo"
)
```
with:
```go
import (
    "context"
    "fmt"
    "net/http"
    "os"
    "strings"
    "text/tabwriter"
    "time"
)
```

And add local view types at the top of the file (after the import block):
```go
// workspaceView is the CLI-local representation of a workspace.
// Fields match the JSON keys returned by /api/v1/workspaces/list.
type workspaceView struct {
    ID          string `json:"id"`
    Slug        string `json:"slug"`
    Namespace   string `json:"namespace"`
    DisplayName string `json:"displayName"`
    NodeCount   int64  `json:"nodeCount"`
    Status      string `json:"status"`
}

// policyView is the CLI-local representation of a policy.
type policyView struct {
    Name        string   `json:"name"`
    Action      string   `json:"action"`
    Description string   `json:"description"`
    Namespace   string   `json:"namespace"`
    PolicyTypes []string `json:"policyTypes"`
}

// tokenView is the CLI-local representation of an enrollment token.
type tokenView struct {
    Token      string    `json:"token"`
    Namespace  string    `json:"namespace"`
    UsageLimit int       `json:"usageLimit"`
    Expiry     time.Time `json:"expiry"`
}
```

- [ ] **Step 3: Update all uses of vo.* types in admin.go**

Replace every `vo.WorkspaceVo` → `workspaceView`, `vo.PolicyVo` → `policyView`, `*vo.TokenVo` → `*tokenView` throughout the file.

The `resolveWorkspaceID` function becomes:
```go
func (c *Client) resolveWorkspaceID(namespace string) (string, error) {
    var list []workspaceView
    if err := c.do(context.Background(), http.MethodGet, "/api/v1/workspaces/list", "", nil, &list); err != nil {
        return "", fmt.Errorf("listing workspaces: %w", err)
    }
    for _, ws := range list {
        if ws.Namespace == namespace {
            return ws.ID, nil
        }
    }
    return "", fmt.Errorf("workspace with namespace %q not found", namespace)
}
```

`AddWorkspace`:
```go
func (c *Client) AddWorkspace(slug, namespace, displayName string) error {
    var ws workspaceView
    err := c.do(context.Background(), http.MethodPost, "/api/v1/workspaces/add", "", map[string]string{
        "slug":        slug,
        "namespace":   namespace,
        "displayName": displayName,
    }, &ws)
    if err != nil {
        return err
    }
    fmt.Printf("workspace created\n")
    fmt.Printf("  name:      %s\n", ws.Slug)
    fmt.Printf("  namespace: %s\n", ws.Namespace)
    fmt.Printf("  status:    %s\n", ws.Status)
    fmt.Printf("\nUse -n %s for token/policy commands targeting this workspace.\n", ws.Namespace)
    return nil
}
```

`ListWorkspaces`:
```go
func (c *Client) ListWorkspaces() error {
    var list []workspaceView
    if err := c.do(context.Background(), http.MethodGet, "/api/v1/workspaces/list", "", nil, &list); err != nil {
        return err
    }
    if len(list) == 0 {
        fmt.Println("no workspaces found")
        return nil
    }
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
    fmt.Fprintln(w, "NAME\tNAMESPACE\tDISPLAY-NAME\tNODES\tSTATUS") //nolint:errcheck
    for _, ws := range list {
        fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", //nolint:errcheck
            ws.Slug, ws.Namespace, ws.DisplayName, ws.NodeCount, ws.Status)
    }
    return w.Flush()
}
```

`AddPolicy`:
```go
func (c *Client) AddPolicy(namespace, name, action, description string) error {
    wsID, err := c.resolveWorkspaceID(namespace)
    if err != nil {
        return err
    }
    var p policyView
    err = c.do(context.Background(), http.MethodPost, "/api/v1/policies/create", wsID, map[string]string{
        "name":        name,
        "namespace":   namespace,
        "action":      action,
        "description": description,
    }, &p)
    if err != nil {
        return err
    }
    fmt.Printf("policy %q applied\n", p.Name)
    fmt.Printf("  action:  %s\n", p.Action)
    fmt.Printf("  types:   %s\n", strings.Join(p.PolicyTypes, ", "))
    return nil
}
```

`ListPolicies`:
```go
func (c *Client) ListPolicies(namespace string) error {
    wsID, err := c.resolveWorkspaceID(namespace)
    if err != nil {
        return err
    }
    var list []policyView
    if err := c.do(context.Background(), http.MethodGet, "/api/v1/policies/list", wsID, nil, &list); err != nil {
        return err
    }
    if len(list) == 0 {
        fmt.Printf("no policies in namespace %q\n", namespace)
        return nil
    }
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
    fmt.Fprintln(w, "NAME\tACTION\tTYPES\tDESCRIPTION") //nolint:errcheck
    for _, p := range list {
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", //nolint:errcheck
            p.Name, p.Action, strings.Join(p.PolicyTypes, ","), p.Description)
    }
    return w.Flush()
}
```

`ListTokens`:
```go
func (c *Client) ListTokens(namespace string) error {
    wsID, err := c.resolveWorkspaceID(namespace)
    if err != nil {
        return err
    }
    var list []*tokenView
    if err := c.do(context.Background(), http.MethodGet, "/api/v1/token/list", wsID, nil, &list); err != nil {
        return err
    }
    if len(list) == 0 {
        fmt.Println("no tokens found")
        return nil
    }
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
    fmt.Fprintln(w, "TOKEN\tNAMESPACE\tLIMIT\tEXPIRY") //nolint:errcheck
    for _, t := range list {
        expiry := "never"
        if !t.Expiry.IsZero() {
            expiry = t.Expiry.Format("2006-01-02 15:04")
        }
        limit := fmt.Sprintf("%d", t.UsageLimit)
        if t.UsageLimit == 0 {
            limit = "unlimited"
        }
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Token, t.Namespace, limit, expiry) //nolint:errcheck
    }
    return w.Flush()
}
```

- [ ] **Step 4: Build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 5: Confirm gorm/redis/gin/k8s are gone from agent deps**

```bash
go list -deps ./cmd/lattice/ | grep -E "gorm|redis|gin|k8s\.io/apimachinery"
```
Expected: **no output** — these packages should be fully removed.

- [ ] **Step 6: Check binary size improvement**

```bash
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /tmp/lattice-optimized ./cmd/lattice/
ls -lh /tmp/lattice-optimized
```
Expected: ~19-22MB (down from ~37MB).

- [ ] **Step 7: Lint**

```bash
make lint
```
Expected: no errors.

- [ ] **Step 8: Run all tests**

```bash
make test
```
Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add internal/agent/client/admin.go
git commit -s -m "refactor: replace server/vo types with local view structs in agent/client"
```

---

## Verification Checklist

After all tasks are complete, run these checks:

```bash
# 1. No heavy server deps in agent
go list -deps ./cmd/lattice/ | grep -E "gorm|redis|gin-gonic|k8s\.io/apimachinery"
# Expected: empty

# 2. Binary size (Linux cross-compile)
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /tmp/lattice-final ./cmd/lattice/
ls -lh /tmp/lattice-final
# Expected: ~19-22MB

# 3. All tests pass
make test

# 4. Lint clean
make lint

# 5. latticed still builds (server-side unchanged functionally)
make build SERVICE=latticed
```
