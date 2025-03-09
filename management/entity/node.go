package entity

import (
	"gorm.io/gorm"
	"time"
)

type GroupRoleType string

const (
	AdminRole  GroupRoleType = "admin"
	MemberRole GroupRoleType = "member"
)

// TODO may be use
func handleRole(role GroupRoleType) {
	switch role {
	case AdminRole:
		// deal admin role
	case MemberRole:
		// deal member role
	}
}

// Node full node structure
type Node struct {
	gorm.Model
	Name                string `gorm:"column:name;size:20" json:"name"`
	Description         string `gorm:"column:description;size:255" json:"description"`
	GroupID             uint   `gorm:"column:group_id;size:20" json:"groupID"`     // belong to which group
	CreatedBy           uint   `gorm:"column:created_by;size:64" json:"createdBy"` // ownerID
	UserID              int64  `gorm:"column:user_id" json:"user_id"`
	Hostname            string `gorm:"column:hostname;size:50" json:"hostname"`
	AppID               string `gorm:"column:app_id;size:20" json:"app_id"`
	Address             string `gorm:"column:address;size:50" json:"address"`
	Endpoint            string `gorm:"column:endpoint;size:50" json:"endpoint"`
	PersistentKeepalive int    `gorm:"column:persistent_keepalive" json:"persistent_keepalive"`
	PublicKey           string `gorm:"column:public_key;size:50" json:"public_key"`
	PrivateKey          string `gorm:"column:private_key;size:50" json:"private_key"`
	AllowedIPs          string `gorm:"column:allowed_ips;size:50" json:"allowed_ips"`
	RelayIP             string `gorm:"column:relay_ip;size:100" json:"relay_ip"`
	TieBreaker          int64  `gorm:"column:tie_breaker" json:"tie_breaker"`
	Ufrag               string `gorm:"column:ufrag;size:30" json:"ufrag"`
	Pwd                 string `gorm:"column:pwd;size:50" json:"pwd"`
	Port                int    `gorm:"column:port" json:"port"`
	Status              int    `gorm:"column:status" json:"status"`
}

func (Node) TableName() string {
	return "la_node"
}

// NodeGroup a node may be in multi groups
type NodeGroup struct {
	gorm.Model
	Name        string `gorm:"column:name;size:64" json:"name"`
	Description string `gorm:"column:description;size:255" json:"description"`
	OwnerID     uint   `gorm:"column:owner_id;size:20" json:"ownerId"`
	IsPublic    bool   `gorm:"column:is_public" json:"isPublic"`
	CreatedBy   string `gorm:"column:created_by;size:64" json:"createdBy"`
	UpdatedBy   string `gorm:"column:updated_by;size:64" json:"updatedBy"`
}

func (NodeGroup) TableName() string {
	return "la_group"
}

// GroupMember relationship between Group and Member
type GroupMember struct {
	ID        string    `gorm:"column:id;size:64" json:"id"`
	GroupID   uint      `gorm:"column:group_id;size:20" json:"groupId"`
	GroupName string    `gorm:"column:group_name;size:64" json:"groupName"`
	UserID    uint      `gorm:"column:user_id;size:20" json:"userId"`
	Username  string    `gorm:"column:username;size:64" json:"username"`
	CreatedBy string    `gorm:"column:created_by;size:64" json:"createdBy"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedBy string    `gorm:"column:updated_by;size:64" json:"updatedBy"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
	Role      string    `gorm:"column:role;size:20" json:"role"`     // role：owner, admin, member
	Status    string    `gorm:"column:status;size:20" json:"status"` // status：pending, accepted, rejected
}

func (GroupMember) TableName() string {
	return "la_group_member"
}

// GroupNode relationship between Group and Node
type GroupNode struct {
	ID        string    `gorm:"column:id;size:64" json:"id"`
	GroupID   uint      `gorm:"column:group_id;size:20" json:"groupId"`
	NodeID    uint      `gorm:"column:node_id;size:20" json:"nodeId"`
	GroupName string    `gorm:"column:group_name;size:64" json:"groupName"`
	NodeName  string    `gorm:"column:node_name;size:64" json:"nodeName"`
	CreatedBy string    `gorm:"column:created_by;size:64" json:"createdBy"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (GroupNode) TableName() string {
	return "la_group_node"
}
