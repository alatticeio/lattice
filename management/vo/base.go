package vo

import "time"

type ModelVo struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	DeletedAt time.Time `json:"deletedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
