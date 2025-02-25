package controller

import (
	"fmt"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/pkg/log"
)

type PlanController struct {
	logger     *log.Logger
	planMapper service.PlanService
}

func NewPlanController(planMapper service.PlanService) *PlanController {
	return &PlanController{planMapper: planMapper, logger: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "plan-controller"))}
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
