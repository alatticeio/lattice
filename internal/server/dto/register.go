package dto

type UserDto struct {
	Username  string        `json:"username"`
	Email     string        `json:"email"`
	Password  string        `json:"password"`
	Role      WorkspaceRole `json:"role"`
	Namespace string        `json:"namespace"`
}
