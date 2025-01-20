package config

// User is a linkany user, will be used to login and store token in local
// user use token to fetch config from linkany center
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

// NewUser will create a new user
func NewUser(username, password string) *User {
	return &User{
		Username: username,
		Password: password,
	}
}
