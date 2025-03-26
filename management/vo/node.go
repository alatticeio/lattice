package vo

import (
	"gorm.io/gorm"
	"linkany/management/entity"
	"time"
)

type NodeGroupVo struct {
	*GroupRelationVo
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	NodeCount int    `json:"nodeCount"`
	//NodeIdList   []uint         `json:"nodeIdList"` // for tom-select update/add
	//PolicyIdList []uint         `json:"policyIdList"`
	Description string         `json:"description"`
	CreatedAt   time.Time      `json:"createdAt"`
	DeletedAt   gorm.DeletedAt `json:"deletedAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	CreatedBy   string         `json:"createdBy"`
	UpdatedBy   string         `json:"updatedBy"`
}

// GroupRelationVo for tom-select show
type GroupRelationVo struct {
	*PolicyResourceVo
	*NodeResourceVo
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
	LabelName           string            `json:"labelName"`
}
