package entity

import "gorm.io/gorm"

type Peer struct {
	gorm.Model
	Endpoint            string
	Address             string
	AllowedIPs          string
	PublicKey           string
	PrivateKey          string
	Status              int
	PersistentKeepalive int
}

// TableName returns the table name of the model
func (Peer) TableName() string {
	return "la_peers"
}
