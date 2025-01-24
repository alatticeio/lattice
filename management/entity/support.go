package entity

import "gorm.io/gorm"

// Support is a entity for support
type Support struct {
	gorm.Model
	Name        string
	Username    string
	Description string
}

func (s *Support) TableName() string {
	return "la_supports"
}
