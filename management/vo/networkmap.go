package vo

import "linkany/management/utils"

type NetworkMap struct {
	UserId string
	Peer   *NodeVo
	Peers  []*utils.NodeMessage
}
