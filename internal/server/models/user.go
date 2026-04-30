package models

import "github.com/alatticeio/lattice/internal/server/dto"

// User 结构体：对应用户名密码以及外部 SSO 同步进来的用户
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
	// 关联定义：UserProfile 的主键就是 User 的 ID
	// references:ID 表示引用 User 表的 ID
	// foreignKey:UserID 表示 UserProfile 表里的关联键是 UserID（同时也是主键）
	UserProfile *UserProfile   `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"userProfile,omitempty"`
	Identities  []UserIdentity `gorm:"foreignKey:UserID" json:"identities,omitempty"`
}

func (User) TableName() string {
	return "t_user"
}

// UserProfile 用户详细资料与设置 (与 User 一对一)
type UserProfile struct {
	UserID      string `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	Title       string `gorm:"size:128" json:"title"`
	Company     string `gorm:"size:128" json:"company"`
	Bio         string `gorm:"type:text" json:"bio"`
	Timezone    string `gorm:"size:64;default:'Asia/Shanghai'" json:"timezone"`
	Language    string `gorm:"size:16;default:'zh-CN'" json:"language"`
	EmailNotify bool   `gorm:"default:true" json:"emailNotify"`
}

func (UserProfile) TableName() string {
	return "t_user_profile"
}
