package model

// NodeTo represents a structure that can be used to define relationships or connections
type NodeTo struct {
	Model
	// Name is the name of the node
	Name     string `gorm:"column:name" json:"name"`
	GroupId  uint64 `gorm:"group_id" json:"group_id"`
	NodeId   string `gorm:"node_id" json:"node_id"`
	NodeToId string `gorm:"node_to_id" json:"node_to_id"`            // NodeToId is the identifier of the node this entity is connected to
	AddUser  string `gorm:"column:add_user;size:64" json:"add_user"` // AddUser is the user who added this connection
}
