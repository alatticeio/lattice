package utils

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"linkany/management/entity"
	"time"
)

type TokenInterface interface {
	Generate() (string, error)
	Verify(username, password, token string) (bool, error)

	Parse(token string)
}

var haSalt = []byte("linkany.io")

type Tokener struct {
}

func NewTokener() *Tokener {
	return &Tokener{}
}

func (t *Tokener) Generate(username, password string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"password": password,
		"nbf":      time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	return token.SignedString(haSalt)
}

func (t *Tokener) Verify(username, password, token string) (bool, error) {
	u, err := t.Parse(token)
	if err != nil {
		return false, err
	}

	if u.Username != username || u.Password != password {
		return false, fmt.Errorf("username or password is incorrect")
	}

	return true, nil
}

func (t *Tokener) Parse(tokenString string) (*entity.User, error) {
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
