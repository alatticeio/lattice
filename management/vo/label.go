package vo

import (
	"gorm.io/gorm"
	"time"
)

type LabelVo struct {
	ID          uint64         `json:"id"`
	Label       string         `json:"name"`
	CreatedAt   time.Time      `json:"createdAt"`
	DeletedAt   gorm.DeletedAt `json:"deletedAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	CreatedBy   string         `json:"createdBy"`
	UpdatedBy   string         `json:"updatedBy"`
	Description string         `json:"description"`
}

// NodeLabelVo Node label relation
type NodeLabelVo struct {
	ModelVo
	NodeId    uint64
	LabelId   uint64
	LabelName string
	CreatedBy string
	UpdatedBy string
}
