package entity

// SharedNodeGroup is the entity that represents the shared group
type SharedNodeGroup struct {
	Model
	UserId       uint64
	GroupId      uint64
	GroupName    string
	OwnerId      uint64 `gorm:"column:owner_id;size:20" json:"ownerId"`
	InviteId     uint64
	AcceptStatus AcceptStatus
	Description  string

	GroupNodes    []GroupNode   `gorm:"foreignKey:NetworkID;references:NetworkID"`
	GroupPolicies []GroupPolicy `gorm:"foreignKey:NetworkID;references:NetworkID"`
}

// TableName returns the table name of the shared group
func (SharedNodeGroup) TableName() string {
	return "la_shared_group"
}

// SharedPolicy is the entity that represents the shared policy
type SharedPolicy struct {
	Model
	UserId       uint64
	PolicyId     uint64
	PolicyName   string
	OwnerId      uint64
	InviteId     uint64
	Description  string
	AcceptStatus AcceptStatus
}

// TableName returns the table name of the shared policy
func (SharedPolicy) TableName() string {
	return "la_shared_policy"
}

// SharedNode is the entity that represents the shared node
type SharedNode struct {
	Model
	UserId       uint64
	NodeId       uint64
	Node         Node `gorm:"foreignKey:NodeId"`
	NodeName     string
	OwnerId      uint64
	InviteId     uint64
	AcceptStatus AcceptStatus
	Description  string

	NodeLabels []NodeLabel `gorm:"foreignKey:NodeId;references:NodeId"`
}

func (SharedNode) TableName() string {
	return "la_shared_node"
}

type SharedLabel struct {
	Model
	UserId       uint64
	LabelId      uint64
	LabelName    string
	OwnerId      uint64
	InviteId     uint64
	AcceptStatus AcceptStatus
	Description  string
}

func (SharedLabel) TableName() string {
	return "la_shared_label"
}
