package dto

// UserSettingsResponse 返回给前端的聚合对象
type UserSettingsResponse struct {
	Name        string `json:"name"`        // 对应 User.Username
	Email       string `json:"email"`       // 对应 User.Email
	AvatarURL   string `json:"avatarUrl"`   // 对应 User.AvatarURL
	Title       string `json:"title"`       // 对应 UserProfile.Title
	Company     string `json:"company"`     // 对应 UserProfile.Company
	Bio         string `json:"bio"`         // 对应 UserProfile.Bio
	Timezone    string `json:"timezone"`    // 对应 UserProfile.Timezone
	Language    string `json:"language"`    // 对应 UserProfile.Language
	EmailNotify bool   `json:"emailNotify"` // 对应 UserProfile.EmailNotify
}

// UpdateSettingsRequest 接收前端修改的结构体
type UpdateSettingsRequest struct {
	Name      string `json:"name" binding:"required,min=2"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
	Address   string `json:"address"`
	Gender    int    `json:"gender"`

	Title       string `json:"title"`
	Company     string `json:"company"`
	Bio         string `json:"bio"`
	Timezone    string `json:"timezone"`
	Language    string `json:"language"`
	EmailNotify bool   `json:"emailNotify"`
}
