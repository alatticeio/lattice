package dto

// FeatureFlagItem represents a single feature flag with its current state.
type FeatureFlagItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Group   string `json:"group"`
	Enabled bool   `json:"enabled"`
}

// UpdateFeatureFlagRequest is the body for toggling a feature flag.
type UpdateFeatureFlagRequest struct {
	Enabled bool `json:"enabled"`
}
