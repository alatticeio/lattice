package mapper

import (
	"linkany/control/dto"
	"linkany/control/entity"
)

var (
	_ SupportInterface = (*SupportMapper)(nil)
)

type SupportMapper struct {
	*DatabaseService
}

func (s SupportMapper) List() ([]*entity.Support, error) {
	//TODO implement me
	panic("implement me")
}

func (s SupportMapper) Get() (*entity.Support, error) {
	//TODO implement me
	panic("implement me")
}

func (s SupportMapper) Page() (*entity.Support, error) {
	//TODO implement me
	panic("implement me")
}

func (s SupportMapper) Create(e *dto.SupportDto) (*entity.Support, error) {
	//TODO implement me
	panic("implement me")
}

func NewSupportMapper(db *DatabaseService) *SupportMapper {
	return &SupportMapper{DatabaseService: db}
}
