package model

// Peer full node structure
type Peer struct {
	Model
	Name                string  `gorm:"column:name;size:20" json:"name"`
	Description         string  `gorm:"column:description;size:255" json:"description"`
	CreatedBy           string  `gorm:"column:created_by;size:64" json:"createdBy"` // ownerID
	UserId              uint64  `gorm:"column:user_id" json:"user_id"`
	Hostname            string  `gorm:"column:hostname;size:50" json:"hostname"`
	AppID               string  `gorm:"column:app_id;size:20" json:"app_id"`
	Address             *string `gorm:"column:address;size:50" json:"address"`
	Endpoint            string  `gorm:"column:endpoint;size:50" json:"endpoint"`
	PersistentKeepalive int     `gorm:"column:persistent_keepalive" json:"persistent_keepalive"`
	PublicKey           string  `gorm:"column:public_key;size:50" json:"public_key"`
	PrivateKey          string  `gorm:"column:private_key;size:50" json:"private_key"`
	AllowedIPs          string  `gorm:"column:allowed_ips;size:50" json:"allowed_ips"`
	RelayIP             string  `gorm:"column:relay_ip;size:100" json:"relay_ip"`
	WrrpAddr            string  `gorm:"column:wrrp_addr;size:300" json:"wrrp_addr"` // drp server address, if is drp node
	TieBreaker          uint32  `gorm:"column:tie_breaker" json:"tie_breaker"`
	Ufrag               string  `gorm:"column:ufrag;size:30" json:"ufrag"`
	Owner               string
	Pwd                 string            `gorm:"column:pwd;size:50" json:"pwd"`
	Port                int               `gorm:"column:port" json:"port"`
	Labels              map[string]string `gorm:"labels" json:"labels"`
}

func (Peer) TableName() string {
	return "la_node"
}
