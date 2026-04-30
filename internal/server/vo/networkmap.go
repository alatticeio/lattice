package vo

import (
	"github.com/alatticeio/lattice/internal/agent/infra"
)

type NetworkMap struct {
	UserId  string
	Current *PeerVo
	Nodes   []*infra.Peer
}
