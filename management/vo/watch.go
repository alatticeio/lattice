package vo

import (
	"wireflow/internal/infra"
)

func (vo *PeerVo) TransferToNode() *infra.Peer {
	return &infra.Peer{
		Name:                vo.Name,
		Description:         vo.Description,
		NetworkId:           vo.NetworkID,
		CreatedBy:           vo.CreatedBy,
		UserId:              vo.UserId,
		Hostname:            vo.Hostname,
		AppID:               vo.AppID,
		Address:             vo.Address,
		Endpoint:            vo.Endpoint,
		PersistentKeepalive: vo.PersistentKeepalive,
		PublicKey:           vo.PublicKey,
		AllowedIPs:          vo.AllowedIPs,
	}
}
