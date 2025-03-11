package dto

import (
	"gorm.io/gorm"
	"linkany/management/entity"
	"time"
)

// UserDto is a data transfer object for User entity
type UserDto struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NodeDto is a data transfer object for Peer entity
type NodeDto struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	UserID              int64     `gorm:"column:user_id" json:"user_id"`
	Name                string    `gorm:"column:name;size:20" json:"name"`
	Hostname            string    `gorm:"column:hostname;size:50" json:"hostname"`
	Description         string    `gorm:"column:description;size:255" json:"description"`
	AppID               string    `gorm:"column:app_id;size:20" json:"app_id"`
	Address             string    `gorm:"column:address;size:50" json:"address"`
	Endpoint            string    `gorm:"column:endpoint;size:50" json:"endpoint"`
	PersistentKeepalive int       `gorm:"column:persistent_keepalive" json:"persistent_keepalive"`
	PublicKey           string    `gorm:"column:public_key;size:50" json:"public_key"`
	PrivateKey          string    `gorm:"column:private_key;size:50" json:"private_key"`
	AllowedIPs          string    `gorm:"column:allowed_ips;size:50" json:"allowed_ips"`
	RelayIP             string    `gorm:"column:relay_ip;size:100" json:"relay_ip"`
	TieBreaker          int64     `gorm:"column:tie_breaker" json:"tie_breaker"`
	UpdatedAt           time.Time `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt           time.Time `gorm:"column:deleted_at;default:NULL" json:"deleted_at"`
	CreatedAt           time.Time `gorm:"column:created_at" json:"created_at"`
	Ufrag               string    `gorm:"column:ufrag;size:30" json:"ufrag"`
	Pwd                 string    `gorm:"column:pwd;size:50" json:"pwd"`
	Port                int       `gorm:"column:port" json:"port"`
	Status              int       `gorm:"column:status" json:"status"`
}

// PlanDto is a data transfer object for Plan entity
type PlanDto struct {
}

// SupportDto is a data transfer object for Support entity
type SupportDto struct {
}

type InviteDto struct {
	Username       string
	InviteUsername string
	InvitationId   int64
	InviterId      int64
	MobilePhone    string
	Email          string
	Permissions    string
	Group          string
	Network        string // 192.168.0.0/24
}

type NodeGroupDto struct {
	gorm.Model
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       uint64 `json:"ownerId"`
	IsPublic    bool   `json:"isPublic"`
	CreatedBy   string `json:"createdBy"`
	UpdatedBy   string `json:"updatedBy"`

	NodeArray   string   `json:"nodeArray"`
	PolicyArray string   `json:"policyArray"`
	Nodes       []string `json:"nodes"`
	Policies    []string `json:"policies"`
}

type GroupMemberDto struct {
	gorm.Model
	ID        int64  `json:"id"`
	GroupID   uint   `json:"groupID"`
	GroupName string `json:"groupName"`
	UserID    uint   `json:"userID"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedBy string `json:"createdBy"`
	UpdatedBy string `json:"updatedBy"`
}

type GroupNodeDto struct {
	gorm.Model
	GroupID   uint   `json:"groupID"`
	GroupName string `json:"groupName"`
	NodeID    uint   `json:"nodeID"`
	NodeName  string `json:"nodeName"`
	CreatedBy string `json:"createdBy"`
}

type AccessPolicyDto struct {
	gorm.Model
	Name        string        `json:"name"`       // 策略名称
	GroupID     uint          `json:"group_id"`   // 所属分组
	Priority    int           `json:"priority"`   // 策略优先级（数字越大优先级越高）
	Effect      string        `json:"effect"`     // 效果：allow/deny
	Status      entity.Status `json:"status"`     // 策略状态：启用/禁用
	CreatedBy   string        `json:"created_by"` // 创建者
	UpdatedBy   string
	Description string
}

type AccessRuleDto struct {
	gorm.Model
	PolicyID   uint      `json:"policyId"`             // 所属策略ID
	SourceType string    `json:"sourceType"`           // 源类型：node/label/all
	SourceID   string    `json:"sourceId"`             // 源标识（节点ID或标签）
	TargetType string    `json:"targetType"`           // 目标类型：node/label/all
	TargetID   string    `json:"targetId"`             // 目标标识（节点ID或标签）
	Actions    string    `json:"actions"`              // 允许的操作列表
	Conditions Condition `json:"conditions,omitempty"` // 额外条件（如时间限制、带宽限制等）
}

type Condition struct {
	MaxBandwidth string `json:"maxBandwidth"`
	TimeWindow   struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
}

type TagDto struct {
	gorm.Model
	Label     string `json:"label"`
	OwnerId   uint64 `json:"ownerId"`
	CreatedBy string `json:"createdBy"`
	UpdatedBy string `json:"updatedBy"`
}
