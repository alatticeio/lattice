package entity

// NetworkMap is a entity for network map, a map belong to a user
type NetworkMap struct {
	UserId string

	Peer *Peer //current peer

	Peers []*Peer //all others peers
}
