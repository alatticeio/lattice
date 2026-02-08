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
	Subject string `json:"sub"`
	Email   string `json:"email"`
	// 增加当前选中的团队 ID，方便实现“Vercel 风格”的上下文切换
	TeamID string `json:"tid"`
	jwt.RegisteredClaims
}
