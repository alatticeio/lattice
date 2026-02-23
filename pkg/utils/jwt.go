package utils

import (
	"errors"
	"fmt"
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
func ParseToken(tokenString string) (*model.WireFlowClaims, error) {
	// 1. 准备载体：直接声明目标结构体指针
	claims := &model.WireFlowClaims{}

	// 2. 解析 Token
	// 注意：第二个参数传入声明好的 claims，库会自动填充数据
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {

		// 3. 安全校验：确保签名算法是 HS256（防止 alg:none 攻击）
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名算法: %v", token.Header["alg"])
		}

		// 4. 获取密钥：必须与 GenerateBusinessJWT 保持一致
		// 强制转换为 []byte 是避免 "signature is invalid" 的核心
		secret := GetJWTSecret()

		// 如果 GetJWTSecret 返回的是 string，转为 []byte
		//if s, ok := secret.(string); ok {
		//	return []byte(s), nil
		//}
		// 如果已经是 []byte，直接返回
		return secret, nil
	})

	// 5. 处理解析过程中的错误（如过期、篡改、格式错误）
	if err != nil {
		return nil, err
	}

	// 6. 最终验证：只有 Valid 为 true 且断言成功才返回数据
	// 此时 claims 已经被 ParseWithClaims 填充满了
	if token.Valid {
		return claims, nil
	}

	return nil, errors.New("Token 验证失败：无效的凭证")
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
		Name:    email,
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
