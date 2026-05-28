package dto

// UserSettingsResponse is the aggregate object returned to the frontend
type UserSettingsResponse struct {
	Name         string `json:"name"`         // Maps to User.Username
	Email        string `json:"email"`        // Maps to User.Email
	AvatarURL    string `json:"avatarUrl"`    // Maps to User.AvatarURL
	Title        string `json:"title"`        // Maps to UserProfile.Title
	Company      string `json:"company"`      // Maps to UserProfile.Company
	Bio          string `json:"bio"`          // Maps to UserProfile.Bio
	Timezone     string `json:"timezone"`     // Maps to UserProfile.Timezone
	Language     string `json:"language"`     // Maps to UserProfile.Language
	EmailNotify  bool   `json:"emailNotify"`  // Maps to UserProfile.EmailNotify
	EnforcerMode string `json:"enforcerMode"` // "auto", "iptables", "ebpf"
}

// UpdateSettingsRequest receives the update structure from the frontend
type UpdateSettingsRequest struct {
	Name      string `json:"name" binding:"required,min=2"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
	Address   string `json:"address"`
	Gender    int    `json:"gender"`

	Title        string `json:"title"`
	Company      string `json:"company"`
	Bio          string `json:"bio"`
	Timezone     string `json:"timezone"`
	Language     string `json:"language"`
	EmailNotify  bool   `json:"emailNotify"`
	EnforcerMode string `json:"enforcerMode"`
}
