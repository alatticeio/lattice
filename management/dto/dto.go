package dto

import (
	"time"
	"wireflow/internal"
	"wireflow/management/entity"
	"wireflow/pkg/utils"

	"gorm.io/gorm"
)

// UserDto is a data transfer object for User entity
type UserDto struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NodeDto is a data transfer object for Current entity
type NodeDto struct {
	ID                  uint64               `gorm:"primaryKey;autoIncrement;column:id" json:"id,string"`
	UserID              uint64               `gorm:"column:user_id" json:"user_id"`
	Name                string               `gorm:"column:name;size:20" json:"name"`
	Hostname            string               `gorm:"column:hostname;size:50" json:"hostname"`
	Description         string               `gorm:"column:description;size:255" json:"description"`
	AppID               string               `gorm:"column:app_id;size:20" json:"app_id"`
	Address             string               `gorm:"column:address;size:50" json:"address"`
	Endpoint            string               `gorm:"column:endpoint;size:50" json:"endpoint"`
	PersistentKeepalive int                  `gorm:"column:persistent_keepalive" json:"persistent_keepalive"`
	PublicKey           string               `gorm:"column:public_key;size:50" json:"public_key"`
	PrivateKey          string               `gorm:"column:private_key;size:50" json:"private_key"`
	AllowedIPs          string               `gorm:"column:allowed_ips;size:50" json:"allowed_ips"`
	RelayIP             string               `gorm:"column:relay_ip;size:100" json:"relay_ip"`
	TieBreaker          uint32               `gorm:"column:tie_breaker" json:"tie_breaker"`
	UpdatedAt           time.Time            `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt           time.Time            `gorm:"column:deleted_at;default:NULL" json:"deleted_at"`
	CreatedAt           time.Time            `gorm:"column:created_at" json:"created_at"`
	Ufrag               string               `gorm:"column:ufrag;size:30" json:"ufrag"`
	Pwd                 string               `gorm:"column:pwd;size:50" json:"pwd"`
	Port                int                  `gorm:"column:port" json:"port"`
	Status              utils.NodeStatus     `gorm:"column:status" json:"status"`
	ActiveStatus        utils.ActiveStatus   `gorm:"column:active_status;type:int" json:"activeStatus"`
	GroupName           string               `gorm:"column:group_name;size:50" json:"groupName"`
	ConnectType         internal.ConnectType `gorm:"column:connect_type;type:int" json:"connectType"`
	LabelIds            string               `gorm:"column:label_ids;size:50" json:"labelIds"`
}

// PlanDto is a data transfer object for Plan entity
type PlanDto struct {
}

// SupportDto is a data transfer object for Support entity
type SupportDto struct {
}

// InviteDto 邀请者: InviteeId
// 被邀请: InvitationId
// 邀请表主键: InviteId
type InviteDto struct {
	ID               uint64
	InviteeName      string
	InviterName      string
	InvitationId     uint64
	InviteeId        uint64
	MobilePhone      string
	Email            string
	Permissions      string
	GroupIds         string `json:"groupIds"`
	NodeIds          string `json:"nodeIds"`
	PolicyIds        string `json:"policyIds"`
	LabelIds         string `json:"labelIds"`
	PermissionIds    string `json:"permissionIds"`
	GroupIdList      []uint64
	NodeIdList       []uint64
	PolicyIdList     []uint64
	LabelIdList      []uint64
	PermissionIdList []uint64
	Network          string // 192.168.0.0/24
	Description      string
}

type InvitationDto struct {
	ID           uint64
	AcceptStatus entity.AcceptStatus
	Permissions  string
	RoleId       uint64
}

type NodeGroupDto struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Owner       uint64             `json:"ownerId"`
	IsPublic    bool               `json:"isPublic"`
	CreatedBy   string             `json:"createdBy"`
	UpdatedBy   string             `json:"updatedBy"`
	Status      utils.ActiveStatus `json:"status"`

	GroupRelationDto
}

type GroupRelationDto struct {
	NodeIds        string `json:"nodeIds,omitempty"`
	PolicyIds      string `json:"policyIds,omitempty"`
	NodeIdList     []string
	PolicyIdList   []string
	NodeNameList   []string
	PolicyNameList []string
}

type GroupMemberDto struct {
	gorm.Model
	ID        uint64 `json:"id"`
	GroupID   uint64 `json:"groupID"`
	GroupName string `json:"groupName"`
	UserID    uint64 `json:"userID"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedBy string `json:"createdBy"`
	UpdatedBy string `json:"updatedBy"`
}

type GroupNodeDto struct {
	ID          uint64 `json:"id"`
	NetworkId   string `json:"networkId"`
	NetworkName string `json:"networkName"`
	NodeID      uint64 `json:"nodeID"`
	NodeName    string `json:"nodeName"`
	CreatedBy   string `json:"createdBy"`
}

type AccessPolicyDto struct {
	ID          uint64
	Name        string             `json:"name"`       // 策略名称
	GroupID     uint64             `json:"group_id"`   // 所属分组
	Priority    int                `json:"priority"`   // 策略优先级（数字越大优先级越高）
	Effect      string             `json:"effect"`     // 效果：allow/deny
	Status      utils.ActiveStatus `json:"status"`     // 策略状态：启用/禁用
	CreatedBy   string             `json:"created_by"` // 创建者
	UpdatedBy   string
	Description string
}

type AccessRuleDto struct {
	ID         uint64
	PolicyID   uint64             `json:"policyId"`   // 所属策略ID
	SourceType string             `json:"sourceType"` // 源类型：node/label/all
	SourceID   string             `json:"sourceId"`   // 源标识（节点ID或标签）
	TargetType string             `json:"targetType"` // 目标类型：node/label/all
	TargetID   string             `json:"targetId"`   // 目标标识（节点ID或标签）
	Actions    string             `json:"actions"`    // 允许的操作列表
	Status     utils.ActiveStatus `json:"status"`
	Conditions Condition          `json:"conditions,omitempty"` // 额外条件（如时间限制、带宽限制等）
}

type Condition struct {
	MaxBandwidth string `json:"maxBandwidth"`
	TimeWindow   struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
}

type TagDto struct {
	ID        uint64
	Label     string `json:"label"`
	OwnerId   uint64 `json:"ownerId"`
	CreatedBy string `json:"createdBy"`
	UpdatedBy string `json:"updatedBy"`
}

type NodeLabelDto struct {
	ID        uint64
	LabelID   uint64 `json:"labelId"`
	LabelName string `json:"labelName"`
	NodeID    uint64 `json:"nodeId"`
	CreatedBy string `json:"createdBy"`
	UpdatedBy string `json:"updatedBy"`
}

type NodeLabelUpdateReq struct {
	Appid         string `json:"appid"`
	LabelEditData struct {
		Text  []string `json:"text"`
		Value []string `json:"value"`
	} `json:"labelEditData"`
	Id        string `json:"id"`
	LabelIds  string `json:"labelIds"`
	CreatedBy string `json:"createdBy"`
}
