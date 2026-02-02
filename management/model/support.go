package model

// Support is a entity for support
type Support struct {
	Model
	Name        string
	Username    string
	Description string
}

func (s *Support) TableName() string {
	return "la_supports"
}
