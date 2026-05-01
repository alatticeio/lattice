package models

// CustomMetric stores user-defined PromQL queries.
type CustomMetric struct {
	Model

	Name        string `gorm:"not null" json:"name"`
	WorkspaceID string `gorm:"index;not null" json:"workspace_id"`
	Query       string `gorm:"type:text;not null" json:"query"`
	Type        string `gorm:"not null" json:"type"`
	ResultType  string `gorm:"not null" json:"result_type"`
	Labels      string `gorm:"type:text" json:"labels"`
	CreatedBy   string `json:"created_by"`
}
