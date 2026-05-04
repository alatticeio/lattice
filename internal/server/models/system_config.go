package models

import (
	"time"
)

const (
	ConfigKeyNatsURL = "nats_url"
)

type SystemConfig struct {
	Key       string    `gorm:"primaryKey;type:varchar(128);not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemConfig) TableName() string { return "la_system_config" }
