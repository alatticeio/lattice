package model

type Plan struct {
	Model
	Name        string
	Price       float64
	Description string
}

func (p *Plan) TableName() string {
	return "la_plans"
}
