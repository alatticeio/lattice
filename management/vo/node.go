package vo

import (
	"gorm.io/gorm"
	"time"
)

type NodeGroupVo struct {
	*GroupRelationVo
	ID          uint           `json:"id"`
	Name        string         `json:"name"`
	NodeCount   int            `json:"nodeCount"`
	Description string         `json:"description"`
	CreatedAt   time.Time      `json:"createdAt"`
	DeletedAt   gorm.DeletedAt `json:"deletedAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	CreatedBy   string         `json:"createdBy"`
	UpdatedBy   string         `json:"updatedBy"`
}

type GroupRelationVo struct {
	NodeIds     []string `json:"nodeIds"`
	PolicyIds   []string `json:"policyIds"`
	NodeNames   []string `json:"nodeNames"`
	PolicyNames []string `json:"policyNames"`
}

type NodeVo struct {
	ID                  uint   `json:"id,string"`
	Name                string `json:"name,omitempty"`
	Description         string `json:"description,omitempty"`
	GroupID             uint   `json:"groupID,omitempty"`   // belong to which group
	CreatedBy           uint   `json:"createdBy,omitempty"` // ownerID
	UserID              int64  `json:"userId,omitempty"`
	Hostname            string `json:"hostname,omitempty"`
	AppID               string `json:"appId,omitempty"`
	Address             string `json:"address,omitempty"`
	Endpoint            string `json:"endpoint,omitempty"`
	PersistentKeepalive int    `json:"persistentKeepalive,omitempty"`
	PublicKey           string `json:"publicKey,omitempty"`
	AllowedIPs          string `json:"allowedIps,omitempty"`
	RelayIP             string `json:"relayIp,omitempty"`
	TieBreaker          int64  `json:"tieBreaker"`
	Ufrag               string `json:"ufrag"`
	Pwd                 string `json:"pwd"`
	Port                int    `json:"port"`
	Status              int    `json:"status"`
}
