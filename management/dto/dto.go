package dto

import "time"

// UserDto is a data transfer object for User entity
type UserDto struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// PeerDto is a data transfer object for Peer entity
type PeerDto struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	InstanceID          int64     `gorm:"column:instance_id" json:"instance_id"`
	UserID              int64     `gorm:"column:user_id" json:"user_id"`
	Name                string    `gorm:"column:name;size:20" json:"name"`
	Hostname            string    `gorm:"column:hostname;size:50" json:"hostname"`
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
	InvitationId int64
	InviterId    int64
	Email        string
	Network      string // 192.168.0.0/24
}
