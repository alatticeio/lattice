package vo

import (
	"wireflow/internal/core/domain"
)

type NetworkMap struct {
	UserId  string
	Current *PeerVO
	Nodes   []*domain.Peer
}
