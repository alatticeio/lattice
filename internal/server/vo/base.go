package vo

import "time"

type ModelVo struct {
	ID        uint64    `json:"id,string"`
	CreatedAt time.Time `json:"createdAt"`
	DeletedAt time.Time `json:"deletedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
