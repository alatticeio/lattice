package vo

import (
	"wireflow/internal/infra"
)

type NetworkMap struct {
	UserId  string
	Current *PeerVO
	Nodes   []*infra.Peer
}
