package entity

type NetworkMap struct {
	UserId string
	Peer   *Peer
	Peers  []*Peer
}
