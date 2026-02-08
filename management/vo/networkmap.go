package vo

import (
	"wireflow/internal/infra"
)

type NetworkMap struct {
	UserId  string
	Current *PeerVo
	Nodes   []*infra.Peer
}
