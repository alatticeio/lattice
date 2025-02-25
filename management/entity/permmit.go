package entity

import "gorm.io/gorm"

// Permission user permit，user's all permit will record in this table
type Permission struct {
	gorm.Model
	ResourceType string `json:"resource_type"` //group,node,policy
	ResourceId   uint   `json:"resource_id"`   //group1.id， on group one record
	UserID       uint   `json:"user_id"`
	Permissions  string `json:"permissions"` // group:create,delete,update,view;node:add,remove,update,connect; policy:add,remove,update,connect
}

func (Permission) TableName() string {
	return "la_user_permission"
}
