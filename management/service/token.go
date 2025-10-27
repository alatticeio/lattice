package service

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"time"
	"wireflow/management/entity"
	"wireflow/management/repository"
	"wireflow/pkg/log"
)

type TokenService interface {
	Generate(username, password string) (string, error)
	Verify(ctx context.Context, username, password string) (bool, *entity.User, error)

	Parse(token string) (*entity.User, error)
}

var haSalt = []byte("wireflow.io")

type tokenServiceImpl struct {
	logger   *log.Logger
	db       *gorm.DB
	userRepo repository.UserRepository
}

func NewTokenService(db *gorm.DB) TokenService {
	return &tokenServiceImpl{
		db: db, logger: log.NewLogger(log.Loglevel, "token-service"),
		userRepo: repository.NewUserRepository(db),
	}
}

func (t *tokenServiceImpl) Generate(username, password string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"password": password,
		"nbf":      time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	return token.SignedString(haSalt)
}

func (t *tokenServiceImpl) Verify(ctx context.Context, username, password string) (bool, *entity.User, error) {

	user, err := t.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return false, nil, err
	}
	if user.Username != username {
		return false, nil, fmt.Errorf("user %s not found", username)
	}

	if user.Password != password {
		return false, nil, fmt.Errorf("password not match")
	}

	return true, user, nil
}

func (t *tokenServiceImpl) Parse(tokenString string) (*entity.User, error) {
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
