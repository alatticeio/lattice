package entity

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Gender   int    `json:"gender,omitempty"`
}

func (u *User) TableName() string {
	return "la_user"
}

type Token struct {
	Token string `json:"token,omitempty"`
}
