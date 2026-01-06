package vo

import (
	"wireflow/internal/core/infra"
)

type NetworkMap struct {
	UserId  string
	Current *PeerVO
	Nodes   []*infra.Peer
}
