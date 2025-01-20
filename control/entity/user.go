package entity

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	UserName string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Gender   int    `json:"gender,omitempty"`
}

type Token struct {
	Token string `json:"token,omitempty"`
}
