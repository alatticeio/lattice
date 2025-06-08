package vo

import (
	"linkany/management/utils"
)

type NetworkMap struct {
	UserId  string
	Current *NodeVo
	Nodes   []*utils.NodeMessage
}
