package dto

type UserDto struct {
	Username    string        `json:"username"  binding:"required,min=2,max=64"`
	Email       string        `json:"email"     binding:"required,email,max=128"`
	Password    string        `json:"password"  binding:"required,min=8,max=128"`
	Role        WorkspaceRole `json:"role"`
	Namespace   string        `json:"namespace"`
	TOSAccepted bool          `json:"tosAccepted"`
}
