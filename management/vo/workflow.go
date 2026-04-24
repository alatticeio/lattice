package vo

// InvitePreviewVo is returned by the public invite preview endpoint.
type InvitePreviewVo struct {
	Email         string `json:"email"`
	WorkspaceID   string `json:"workspaceId"`
	WorkspaceName string `json:"workspaceName"`
	InviterName   string `json:"inviterName"`
	InviterEmail  string `json:"inviterEmail"`
	Role          string `json:"role"`
	ExpiresAt     string `json:"expiresAt"`
	Status        string `json:"status"`
}

// WorkflowRequestVo is the HTTP response shape for a workflow approval request.
type WorkflowRequestVo struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"createdAt"`
	WorkspaceID string `json:"workspaceId"`

	RequestedBy      string `json:"requestedBy"`
	RequestedByName  string `json:"requestedByName"`
	RequestedByEmail string `json:"requestedByEmail"`

	ResourceType string `json:"resourceType"`
	ResourceName string `json:"resourceName"`
	Action       string `json:"action"`

	Status string `json:"status"`

	ReviewedBy     string  `json:"reviewedBy,omitempty"`
	ReviewedByName string  `json:"reviewedByName,omitempty"`
	ReviewedAt     *string `json:"reviewedAt,omitempty"`
	ReviewNote     string  `json:"reviewNote,omitempty"`

	ExecutedAt   *string `json:"executedAt,omitempty"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
}
