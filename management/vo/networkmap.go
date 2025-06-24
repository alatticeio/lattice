package vo

import (
	"linkany/internal"
)

type NetworkMap struct {
	UserId  string
	Current *NodeVo
	Nodes   []*internal.NodeMessage
}
