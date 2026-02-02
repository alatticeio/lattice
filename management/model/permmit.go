package model

type Permissions struct {
	Model
	ResourceType    string
	Name            string
	PermissionValue string
	Description     string
}

func (Permissions) TableName() string {
	return "la_permissions"
}

// UserPermission user permit，user's all permit will record in this table
type UserPermission struct {
	Model
	ResourceType string `json:"resource_type"` //group,node,policy
	ResourceId   uint   `json:"resource_id"`   //group1.id， on group one record
	UserID       uint64 `json:"user_id"`
	Permissions  string `json:"permissions"` // group:create,delete,update,view;node:add,remove,update,connect; policy:add,remove,update,connect
}

func (UserPermission) TableName() string {
	return "la_user_permission"
}
