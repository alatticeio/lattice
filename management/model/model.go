package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Model a basic GoLang struct which includes the following fields: ID, CreatedAt, UpdatedAt, DeletedAt
// It may be embedded into your model or you may build your own model without it
//
//	type User struct {
//	  gorm.Model
//	}
type Model struct {
	ID        string         `gorm:"primaryKey;type:text;autoIncrement:false"`
	CreatedAt time.Time      `json:"created_at" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type NamespaceAware interface {
	SetNamespace(ns string)
}

// uuid v7 time + rand
func (m *Model) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}

	// 如果当前对象实现了设置 Namespace 的接口，自动同步
	if nsTarget, ok := tx.Statement.Dest.(NamespaceAware); ok {
		nsTarget.SetNamespace("wf-" + m.ID)
	}
	return
}
