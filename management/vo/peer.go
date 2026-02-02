package vo

import (
	"time"
)

type PeerVO struct {
	ID                  uint64    `json:"id,string"`
	Namespace           string    `json:"namespace"`
	Name                string    `json:"name,omitempty"`
	Description         string    `json:"description,omitempty"`
	NetworkID           string    `json:"networkID,omitempty"` // belong to which group
	CreatedBy           string    `json:"createdBy,omitempty"` // ownerID
	UserId              uint64    `json:"userId,omitempty"`
	Platform            string    `json:"platform"`
	Hostname            string    `json:"hostname,omitempty"`
	AppID               string    `json:"appId,omitempty"`
	Address             *string   `json:"address,omitempty"`
	Endpoint            string    `json:"endpoint,omitempty"`
	PersistentKeepalive int       `json:"persistentKeepalive,omitempty"`
	PublicKey           string    `json:"publicKey,omitempty"`
	AllowedIPs          string    `json:"allowedIps,omitempty"`
	RelayIP             string    `json:"relayIp,omitempty"`
	Pwd                 string    `json:"pwd"`
	GroupName           string    `json:"groupName"`
	Version             uint64    `json:"version"`
	LastUpdatedAt       time.Time `json:"lastUpdatedAt"`

	Labels map[string]string `json:"labels,omitempty"`
}
