package controller

import (
	"linkany/control/entity"
	"linkany/control/mapper"
)

type PlanController struct {
	planMapper *mapper.PlanMapper
}

func NewPlanController(planMapper *mapper.PlanMapper) *PlanController {
	return &PlanController{planMapper: planMapper}
}

func (p *PlanController) List() ([]*entity.Plan, error) {
	return p.planMapper.List()
}

func (p *PlanController) Get() (*entity.Plan, error) {
	return p.planMapper.Get()
}

func (p *PlanController) Page() (*entity.Plan, error) {
	return p.planMapper.Page()
}
