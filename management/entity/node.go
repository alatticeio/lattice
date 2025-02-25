package entity

import "gorm.io/gorm"

type GroupRoleType string

const (
	AdminRole  GroupRoleType = "admin"
	MemberRole GroupRoleType = "member"
)

// TODO may be use
func handleRole(role GroupRoleType) {
	switch role {
	case AdminRole:
		// 处理管理员角色
	case MemberRole:
		// 处理成员角色
	}
}

// Node full node structure
type Node struct {
	gorm.Model
	// NodeType indicates the type of the node, e.g., "server", "peer", "client"
	NodeType            string `gorm:"column:node_type;size:20" json:"node_type"`
	Name                string `gorm:"column:name;size:20" json:"name"`
	Description         string `json:"description"`
	GroupID             uint   `json:"group_id"`   // belong to which group
	CreatedBy           uint   `json:"created_by"` // ownerID
	InstanceID          int64  `gorm:"column:instance_id" json:"instance_id"`
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
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     uint   `json:"owner_id"`  // 分组所有者ID
	IsPublic    bool   `json:"is_public"` // 是否公开
}

// 分组成员关系
type GroupMember struct {
	gorm.Model
	GroupID uint   `json:"group_id"`
	NodeID  uint   `json:"node_id"`
	Role    string `json:"role"`   // 成员角色：owner, admin, member
	Status  int    `json:"status"` // 状态：待审核、已通过、已拒绝
}
