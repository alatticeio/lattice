package entity

import "gorm.io/gorm"

type Plan struct {
	gorm.Model
	Name        string
	Price       float64
	Description string
}

func (p *Plan) TableName() string {
	return "la_plans"
}
