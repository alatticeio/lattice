package entity

type NetworkMap struct {
	UserId string
	Peer   *Node
	Peers  []*Node
}
