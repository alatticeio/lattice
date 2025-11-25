package vo

import (
	"wireflow/internal"
)

type NetworkMap struct {
	UserId  string
	Current *NodeVo
	Nodes   []*internal.Peer
}
