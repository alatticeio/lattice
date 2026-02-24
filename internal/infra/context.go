package infra

type ContextKey int

const (
	UserIDKey ContextKey = iota
	WorkspaceKey
	StrictTenantKey
)
