package service

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"linkany/management/entity"
	"linkany/pkg/log"
	"time"
)

type TokenInterface interface {
	Generate() (string, error)
	Verify(username, password, token string) (bool, error)

	Parse(token string)
}

var haSalt = []byte("linkany.io")

type TokenService struct {
	logger *log.Logger
	*DatabaseService
}

func NewTokenService(db *DatabaseService) *TokenService {
	return &TokenService{
		DatabaseService: db, logger: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "token-service")),
	}
}

func (t *TokenService) Generate(username, password string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"password": password,
		"nbf":      time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	return token.SignedString(haSalt)
}

func (t *TokenService) Verify(username, password string) (bool, *entity.User, error) {

	var user entity.User
	if err := t.Where("username = ?", username).Find(&user).Error; err != nil {
		return false, nil, fmt.Errorf("user not found")
	}

	if user.Username != username {
		return false, nil, fmt.Errorf("user %s not found", username)
	}

	if user.Password != password {
		return false, nil, fmt.Errorf("password not match")
	}

	return true, &user, nil
}

func (t *TokenService) Parse(tokenString string) (*entity.User, error) {
	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return haSalt, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		fmt.Println(claims["username"], claims["nbf"])
		return &entity.User{
			Username: claims["username"].(string),
			Password: claims["password"].(string),
		}, nil
	}
	return nil, nil

}
