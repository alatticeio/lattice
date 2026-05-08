package dto

// PageRequest generic pagination request parameters
type PageRequest struct {
	Page      int    `form:"page" json:"page"`           // Page number
	PageSize  int    `form:"pageSize" json:"pageSize"`   // Items per page
	Keyword   string `form:"search" json:"search"`       // Search keyword
	Namespace string `form:"namespace" json:"namespace"` // Namespace/isolation field
	Status    string `form:"status" json:"status"`       // Status filter
}

// PageResult generic paginated response container (uses generic type T)
type PageResult[T any] struct {
	Total    int64 `json:"total"`    // Total count
	Page     int   `json:"page"`     // Current page number
	PageSize int   `json:"pageSize"` // Items per page
	List     []T   `json:"list"`     // Data list, renamed to List for better semantics than Data
}
