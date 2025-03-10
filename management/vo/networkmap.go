package vo

type NetworkMap struct {
	UserId string
	Peer   *NodeVo
	Peers  []*NodeVo
}
