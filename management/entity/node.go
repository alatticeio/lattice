package entity

import (
	"wireflow/internal"
	"wireflow/management/vo"
	"wireflow/pkg/utils"
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
	Model
	Name                string `gorm:"column:name;size:20" json:"name"`
	Description         string `gorm:"column:description;size:255" json:"description"`
	CreatedBy           string `gorm:"column:created_by;size:64" json:"createdBy"` // ownerID
	UserId              uint64 `gorm:"column:user_id" json:"user_id"`
	Hostname            string `gorm:"column:hostname;size:50" json:"hostname"`
	AppID               string `gorm:"column:app_id;size:20" json:"app_id"`
	Address             string `gorm:"column:address;size:50" json:"address"`
	Endpoint            string `gorm:"column:endpoint;size:50" json:"endpoint"`
	PersistentKeepalive int    `gorm:"column:persistent_keepalive" json:"persistent_keepalive"`
	PublicKey           string `gorm:"column:public_key;size:50" json:"public_key"`
	PrivateKey          string `gorm:"column:private_key;size:50" json:"private_key"`
	AllowedIPs          string `gorm:"column:allowed_ips;size:50" json:"allowed_ips"`
	RelayIP             string `gorm:"column:relay_ip;size:100" json:"relay_ip"`
	DrpAddr             string `gorm:"column:drp_addr;size:300" json:"drp_addr"` // drp server address, if is drp node
	TieBreaker          uint32 `gorm:"column:tie_breaker" json:"tie_breaker"`
	Ufrag               string `gorm:"column:ufrag;size:30" json:"ufrag"`
	Owner               string
	Pwd                 string           `gorm:"column:pwd;size:50" json:"pwd"`
	Port                int              `gorm:"column:port" json:"port"`
	Status              utils.NodeStatus `gorm:"type:int;column:status" json:"status"`
	ActiveStatus        utils.ActiveStatus
	ConnectType         internal.ConnType // direct, relay, drp

	Group      GroupNode   `gorm:"foreignKey:NodeId;"`
	NodeLabels []NodeLabel `gorm:"foreignKey:NodeId;"`
}

type ListNode struct {
	Node
	GroupName string `gorm:"column:group_name;size:50" json:"groupName"`
	LabelName string `gorm:"column:labels;size:256" json:"labels"`
}

func (Node) TableName() string {
	return "la_node"
}

// NodeGroup a node may be in multi groups
type NodeGroup struct {
	Model
	NetworkID   string `gorm:"column:network_id;size:20" json:"networkId"`
	Name        string `gorm:"column:name;size:64" json:"name"`
	Description string `gorm:"column:description;size:255" json:"description"`

	OwnId     uint64             `gorm:"column:own_id;size:20" json:"ownerId"`
	Owner     string             `gorm:"column:owner;size:64" json:"owner"`
	IsPublic  bool               `gorm:"column:is_public" json:"isPublic"`
	Status    utils.ActiveStatus `gorm:"column:status" json:"status"` // 0: unapproved, 1: approved, 2: rejected
	CreatedBy string             `gorm:"column:created_by;size:64" json:"createdBy"`
	UpdatedBy string             `gorm:"column:updated_by;size:64" json:"updatedBy"`

	GroupNodes    []GroupNode   `gorm:"foreignKey:NetworkID;"`
	GroupPolicies []GroupPolicy `gorm:"foreignKey:NetworkID;"`
}

func (NodeGroup) TableName() string {
	return "la_group"
}

// GroupMember relationship between GroupVo and Member
type GroupMember struct {
	Model
	GroupID   uint64 `gorm:"column:group_id;size:20" json:"groupId"`
	GroupName string `gorm:"column:group_name;size:64" json:"groupName"`
	UserID    uint64 `gorm:"column:user_id;size:20" json:"userId"`
	Username  string `gorm:"column:username;size:64" json:"username"`
	CreatedBy string `gorm:"column:created_by;size:64" json:"createdBy"`
	UpdatedBy string `gorm:"column:updated_by;size:64" json:"updatedBy"`
	Role      string `gorm:"column:role;size:20" json:"role"`     // role：owner, admin, member
	Status    string `gorm:"column:status;size:20" json:"status"` // status：pending, accepted, rejected
}

func (GroupMember) TableName() string {
	return "la_group_member"
}

// GroupNode relationship between GroupVo and Node
type GroupNode struct {
	Model
	NetworkId string `gorm:"column:network_id;size:20" json:"networkId"`
	NodeId    uint64 `gorm:"column:node_id;size:20" json:"nodeId"`
	GroupName string `gorm:"column:group_name;size:64" json:"groupName"`
	NodeName  string `gorm:"column:node_name;size:64" json:"nodeName"`
	CreatedBy string `gorm:"column:created_by;size:64" json:"createdBy"`
}

func (GroupNode) TableName() string {
	return "la_group_node"
}

// GroupPolicy relationship between GroupVo and Policy
type GroupPolicy struct {
	Model
	GroupId     uint64
	PolicyId    uint64
	PolicyName  string
	Description string
	CreatedBy   string
}

func (GroupPolicy) TableName() string {
	return "la_group_policy"
}

func (node Node) TransferToNodeVo() *vo.NodeVo {
	return &vo.NodeVo{
		ID:                  node.ID,
		Name:                node.Name,
		Description:         node.Description,
		CreatedBy:           node.CreatedBy,
		UserId:              node.UserId,
		Hostname:            node.Hostname,
		AppID:               node.AppID,
		Address:             node.Address,
		Endpoint:            node.Endpoint,
		PersistentKeepalive: node.PersistentKeepalive,
		PublicKey:           node.PublicKey,
		AllowedIPs:          node.AllowedIPs,
		RelayIP:             node.RelayIP,
		TieBreaker:          node.TieBreaker,
		Ufrag:               node.Ufrag,
		Pwd:                 node.Pwd,
		Port:                node.Port,
		Status:              node.Status,
	}
}
