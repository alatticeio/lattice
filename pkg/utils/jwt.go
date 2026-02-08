package utils

import (
	"errors"
	"os"
	"time"
	"wireflow/management/model"

	"github.com/golang-jwt/jwt/v5"
)

var jwtKey = []byte("your_secret_key") // 生产环境请使用环境变量

type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID uint) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // 有效期 24 小时
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// ParseToken 解析并校验 JWT
func ParseToken(tokenString string) (*Claims, error) {
	// 1. 解析 Token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 校验签名算法是否匹配（防止算法降级攻击）
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("意外的签名方法")
		}
		return jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	// 2. 校验 Claims 是否合法以及 Token 是否有效
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("无效的 Token")
}

// 建议从环境变量读取，不要硬编码
//var jwtSecret = []byte("your-256-bit-secret-key-here")

func GetJWTSecret() []byte {
	// 从环境变量获取，比如在 docker-compose 里配置的
	secret := os.Getenv("WF_JWT_SECRET")
	if secret == "" {
		// 生产环境建议在这里直接 panic，强制要求配置 secret
		return []byte("your-256-bit-secret-key-here")
	}
	return []byte(secret)
}

func GenerateBusinessJWT(userID, email string) (string, error) {
	// 1. 设置有效期（例如 12 小时）
	claims := model.WireFlowClaims{
		Subject: userID,
		Email:   email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "wireflow-bff",
			Subject:   userID,
		},
	}

	// 2. 选择签名算法 (HS256 是最常用的对称加密)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 3. 生成最终的字符串 Token
	signedToken, err := token.SignedString(GetJWTSecret())
	if err != nil {
		return "", err
	}

	return signedToken, nil
}
