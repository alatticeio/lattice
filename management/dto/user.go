package dto

type UserNamespacePermissionDto struct {
	UserID      string `gorm:"user_id" json:"user_id"`
	Namespace   string `gorm:"namespace" json:"namespace"`
	AccessLevel string `gorm:"access_level" json:"level"` // "read", "write", "admin"
}
