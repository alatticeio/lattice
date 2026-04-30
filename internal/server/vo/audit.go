package vo

// AuditLogVo is the HTTP response shape for a single audit log entry.
type AuditLogVo struct {
	ID           string `json:"id"`
	CreatedAt    string `json:"createdAt"`
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
	UserEmail    string `json:"userEmail"`
	UserIP       string `json:"userIP"`
	WorkspaceID  string `json:"workspaceId"`
	Action       string `json:"action"`
	Resource     string `json:"resource"`
	ResourceID   string `json:"resourceId"`
	ResourceName string `json:"resourceName"`
	Scope        string `json:"scope"`
	Status       string `json:"status"`
	StatusCode   int    `json:"statusCode"`
	Detail       string `json:"detail,omitempty"`
}
