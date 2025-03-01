package vo

import (
	"time"
)

type LabelVo struct {
	ID        uint      `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
	DeletedAt time.Time `json:"deleted_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy string    `json:"created_by"`
	UpdatedBy string    `json:"updated_by"`
}
