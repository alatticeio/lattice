package models

import "github.com/golang-jwt/jwt/v5"

// WireFlowClaims are the JWT claims issued after successful authentication.
// WorkspaceId is intentionally omitted — workspace context is passed per-request
// via the X-Workspace-Id header.
type WireFlowClaims struct {
	jwt.RegisteredClaims             // sub = userID, exp, iat, iss
	Email      string `json:"email"`
	Username   string `json:"username"`
	SystemRole string `json:"system_role"` // "platform_admin" or "user"
}
