package middleware

import (
	"net/http"
	"strings"

	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"

	"github.com/gin-gonic/gin"
)

const auditScopeKey = "audit_scope"

// SetAuditScope lets a handler override the auto-generated scope description.
// Call this before writing the response.
func SetAuditScope(c *gin.Context, scope string) {
	c.Set(auditScopeKey, scope)
}

// responseCapture wraps gin's ResponseWriter to capture the status code.
type responseCapture struct {
	gin.ResponseWriter
	code int
}

func (r *responseCapture) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseCapture) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// actionFromMethod maps an HTTP method to an audit action verb.
func actionFromMethod(method, path string) string {
	// Special-case login / logout / invite / revoke before falling back to generic verbs.
	lp := strings.ToLower(path)
	if strings.Contains(lp, "/login") {
		return "LOGIN"
	}
	if strings.Contains(lp, "/logout") {
		return "LOGOUT"
	}
	if strings.Contains(lp, "/accept") {
		return "ACCEPT"
	}
	if strings.Contains(lp, "/invitations") && method == http.MethodPost {
		return "INVITE"
	}
	if strings.Contains(lp, "/invitations") && method == http.MethodDelete {
		return "REVOKE"
	}
	switch method {
	case http.MethodPost:
		return "CREATE"
	case http.MethodPut, http.MethodPatch:
		return "UPDATE"
	case http.MethodDelete:
		return "DELETE"
	default:
		return method
	}
}

// resourceFromPath extracts the resource type from the URL path segments.
func resourceFromPath(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	// Walk segments in reverse to find the last meaningful noun
	// (skipping UUID-like segments and known verbs).
	skip := map[string]bool{
		"api": true, "v1": true, "accept": true,
		"login": true, "logout": true, "register": true,
	}
	for i := len(segments) - 1; i >= 0; i-- {
		s := segments[i]
		if s == "" || skip[s] {
			continue
		}
		// Skip UUID-like or numeric-looking segments
		if len(s) == 36 && strings.Count(s, "-") == 4 {
			continue
		}
		// Singularize common plural forms
		return singularize(s)
	}
	return "unknown"
}

// singularize converts simple plural English resource names to singular form.
func singularize(s string) string {
	replacer := strings.NewReplacer(
		"members", "member",
		"invitations", "invitation",
		"workspaces", "workspace",
		"policies", "policy",
		"relays", "relay",
		"tokens", "token",
		"users", "user",
		"peers", "peer",
		"networks", "network",
	)
	result := replacer.Replace(s)
	return result
}

// scopeFromParams builds a default scope string from path parameters.
func scopeFromParams(c *gin.Context) string {
	var parts []string
	if id := c.Param("id"); id != "" {
		parts = append(parts, "workspace:"+id)
	}
	for _, p := range []string{"userID", "invID", "token", "name"} {
		if v := c.Param(p); v != "" {
			parts = append(parts, p+":"+v)
		}
	}
	return strings.Join(parts, " ")
}

// AuditMiddleware returns a Gin middleware that records every non-GET request
// as an audit log entry using the provided AuditService.
func AuditMiddleware(svc service.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only audit mutating requests.
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		// Wrap the writer to capture the status code.
		capture := &responseCapture{ResponseWriter: c.Writer, code: 0}
		c.Writer = capture

		// --- Execute the actual handler ---
		c.Next()
		// ----------------------------------

		// Determine status code (default to 200 if never explicitly set).
		code := capture.code
		if code == 0 {
			code = http.StatusOK
		}

		status := "success"
		if code >= 400 {
			status = "failed"
		}

		// Read scope: handler may have called SetAuditScope.
		scope, _ := c.Get(auditScopeKey)
		scopeStr, _ := scope.(string)
		if scopeStr == "" {
			scopeStr = scopeFromParams(c)
		}

		entry := models.AuditLog{
			UserID:      c.GetString("user_id"),
			UserName:    c.GetString("username"),
			UserEmail:   c.GetString("email"),
			UserIP:      c.ClientIP(),
			WorkspaceID: c.GetHeader("X-Workspace-Id"),
			Action:      actionFromMethod(c.Request.Method, c.FullPath()),
			Resource:    resourceFromPath(c.FullPath()),
			Scope:       scopeStr,
			Status:      status,
			StatusCode:  code,
		}

		svc.Log(entry)
	}
}
