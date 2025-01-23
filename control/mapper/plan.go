package mapper

import "linkany/control/entity"

var (
	_ PlanInterface = (*PlanMapper)(nil)
)

type PlanMapper struct {
	*DatabaseService
}

func NewPlanMapper(db *DatabaseService) *PlanMapper {
	return &PlanMapper{DatabaseService: db}
}

func (p PlanMapper) List() ([]*entity.Plan, error) {
	var plans []*entity.Plan
	if err := p.Where("1=1").Find(&plans).Error; err != nil {
		return nil, err
	}

	return plans, nil
}

func (p PlanMapper) Get() (*entity.Plan, error) {
	//TODO implement me
	panic("implement me")
}

func (p PlanMapper) Page() (*entity.Plan, error) {
	//TODO implement me
	panic("implement me")
}
