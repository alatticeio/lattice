package entity

import (
	"gorm.io/gorm"
	"time"
)

// Model a basic GoLang struct which includes the following fields: ID, CreatedAt, UpdatedAt, DeletedAt
// It may be embedded into your model or you may build your own model without it
//
//	type User struct {
//	  gorm.Model
//	}
type Model struct {
	ID        uint64         `gorm:"primarykey" json:"id,omitempty" :"id"`
	CreatedAt time.Time      `json:"created_at" json:"created_at" :"created_at"`
	UpdatedAt time.Time      `json:"updated_at" :"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at" :"deleted_at"`
}
