package entity

// NetworkMap is a entity for network map, a map belong to a user
type NetworkMap struct {
	userId string

	peer *Peer //current peer

	peers []*Peer //all others peers
}
