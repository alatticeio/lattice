package controller

import (
	"linkany/management/entity"
	"linkany/management/mapper"
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
