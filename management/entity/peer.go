package entity

import "gorm.io/gorm"

type Peer struct {
	gorm.Model
	Endpoint   string
	Address    string
	AllowedIPs string
	PublicKey  string
	PrivateKey string
	Online     int // 0: offline 1: online
}

// TableName returns the table name of the model
func (Peer) TableName() string {
	return "la_peers"
}
