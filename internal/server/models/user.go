package models

import "github.com/alatticeio/lattice/internal/server/dto"

// User struct: represents users with username/password as well as users synced via external SSO
type User struct {
	Model
	SystemRole dto.SystemRole `gorm:"type:varchar(20);column:system_role" json:"systemRole"`
	Username   string         `json:"username,omitempty"`
	Password   string         `json:"password,omitempty"`
	Mobile     string         `json:"mobile,omitempty"`
	Email      string         `json:"email"`
	Avatar     string         `json:"avatar"`
	Address    string         `json:"address,omitempty"`
	Gender     int            `json:"gender,omitempty"`
	// Association: UserProfile's primary key is the User's ID
	// references:ID indicates the referenced column in the User table
	// foreignKey:UserID indicates the association key in the UserProfile table is UserID (which is also the primary key)
	UserProfile *UserProfile   `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"userProfile,omitempty"`
	Identities  []UserIdentity `gorm:"foreignKey:UserID" json:"identities,omitempty"`
}

func (User) TableName() string {
	return "t_user"
}

// UserProfile holds user profile details and settings (one-to-one with User)
type UserProfile struct {
	UserID       string `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	Title        string `gorm:"size:128" json:"title"`
	Company      string `gorm:"size:128" json:"company"`
	Bio          string `gorm:"type:text" json:"bio"`
	Timezone     string `gorm:"size:64;default:'Asia/Shanghai'" json:"timezone"`
	Language     string `gorm:"size:16;default:'zh-CN'" json:"language"`
	EmailNotify  bool   `gorm:"default:true" json:"emailNotify"`
	EnforcerMode string `gorm:"size:16;default:'auto'" json:"enforcerMode"`
}

func (UserProfile) TableName() string {
	return "t_user_profile"
}
