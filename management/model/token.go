package model

import (
	"github.com/golang-jwt/jwt/v5"
)

type Token struct {
	Token      string `json:"token"`
	Namespace  string `json:"namespace"`
	UsageLimit int    `json:"usageLimit"`
	Expiry     string `json:"expiry"`
	//BoundPeers []string `json:"boundPeers,omitempty"`
}

// WireFlowClaims 通常在 Dex 回调成功后，签发一个属于 WireFlow 自己的轻量级 JWT。
type WireFlowClaims struct {
	Subject string `json:"sub"`  // may be userId or extId
	Name    string `json:"name"` // may be username or email
	// 增加当前选中的团队 ID，方便实现“Vercel 风格”的上下文切换
	WorkspaceId string `json:"workspaceId"`
	jwt.RegisteredClaims
}
