package vo

import (
	"linkany/management/entity"
	"time"

	"gorm.io/gorm"
)

type Group struct {
	*GroupRelationVo
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	NodeCount int    `json:"nodeCount"`

	GroupNodes    []entity.GroupNode   `json:"groupNodes"`
	GroupPolicies []entity.GroupPolicy `json:"groupPolicies"`
	Status        string               `json:"status"`
	Description   string               `json:"description"`
	CreatedAt     time.Time            `json:"createdAt"`
	DeletedAt     gorm.DeletedAt       `json:"deletedAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
	CreatedBy     string               `json:"createdBy"`
	UpdatedBy     string               `json:"updatedBy"`
}

// GroupRelationVo for tom-select show
type GroupRelationVo struct {
	*PolicyResourceVo
	*NodeResourceVo
}

func NewGroupRelationVo() *GroupRelationVo {
	return &GroupRelationVo{
		PolicyResourceVo: NewPolicyResourceVo(), // for group policy relation
		NodeResourceVo:   NewNodeResourceVo(),
	}
}

func NewPolicyResourceVo() *PolicyResourceVo {
	return &PolicyResourceVo{
		PolicyValues: make(map[string]string, 1),
	}
}

func NewNodeResourceVo() *NodeResourceVo {
	return &NodeResourceVo{
		NodeValues: make(map[string]string, 1),
	}
}

func NewLabelResourceVo() *LabelResourceVo {
	return &LabelResourceVo{
		LabelValues: make(map[string]string, 1),
	}
}

type NodeVo struct {
	ID                  uint              `json:"id,string"`
	Name                string            `json:"name,omitempty"`
	Description         string            `json:"description,omitempty"`
	GroupID             uint              `json:"groupID,omitempty"`   // belong to which group
	CreatedBy           string            `json:"createdBy,omitempty"` // ownerID
	UserID              uint              `json:"userId,omitempty"`
	Hostname            string            `json:"hostname,omitempty"`
	AppID               string            `json:"appId,omitempty"`
	Address             string            `json:"address,omitempty"`
	Endpoint            string            `json:"endpoint,omitempty"`
	PersistentKeepalive int               `json:"persistentKeepalive,omitempty"`
	PublicKey           string            `json:"publicKey,omitempty"`
	AllowedIPs          string            `json:"allowedIps,omitempty"`
	RelayIP             string            `json:"relayIp,omitempty"`
	TieBreaker          int64             `json:"tieBreaker"`
	Ufrag               string            `json:"ufrag"`
	Pwd                 string            `json:"pwd"`
	Port                int               `json:"port"`
	Status              entity.NodeStatus `json:"status"`
	GroupName           string            `json:"groupName"`
	*LabelResourceVo
}
