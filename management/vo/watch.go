package vo

import "linkany/management/utils"

// Message used to wrapper the message for watch
/*
  a node event message like:
{
	"code": 200,
	"msg": "success",
	"data": {
		"drpUrl": "http://drp.linkany.io/drp",
		"device": {
			"privateKey": "mBngM2k7qWp9pVFGWMO0q1l7tiWjiIIAgsU/jwj+BHU=",
			"publicKey": "3Hyx1Sbq0F9SZc6CUnmJ1pCPMgaAi6JRIxwoTrc1wSA=",
			"address": "10.0.0.4",
			"listenPort": 51820
		},
		"list": [{
			"id": 1866305058815524900,
			"instanceId": 1865958132886462500,
			"userId": 1865418224707231700,
			"name": null,
			"hostname": "VM-4-3-opencloudos",
			"appId": "64d583324d",
			"insPrivateKey": null,
			"insPublicKey": null,
			"address": "10.0.0.4",
			"endpoint": null,
			"persistentKeepalive": 25,
			"publicKey": "lFTblXWiDQTACHfiEqlJ6ORpBMCCiIGER1YgF729xVY=",
			"privateKey": "gEJu/+pPlNa7CFANMvohx12iPf+/XUrpY+F39ntguEc=",
			"allowedIPs": null,
			"hostIp": null,
			"srflxIp": null,
			"relayIp": null,
			"createDate": null
		}]
	}
}
*/

func (vo *NodeVo) TransferToNodeMessage() *utils.NodeMessage {
	return &utils.NodeMessage{
		ID:                  vo.ID,
		Name:                vo.Name,
		Description:         vo.Description,
		GroupID:             vo.GroupID,
		CreatedBy:           vo.CreatedBy,
		UserId:              vo.UserId,
		Hostname:            vo.Hostname,
		AppID:               vo.AppID,
		Address:             vo.Address,
		Endpoint:            vo.Endpoint,
		PersistentKeepalive: vo.PersistentKeepalive,
		PublicKey:           vo.PublicKey,
		AllowedIPs:          vo.AllowedIPs,
		RelayIP:             vo.RelayIP,
	}
}
