package controller

import (
	"linkany/control/dto"
	"linkany/control/entity"
	"linkany/control/mapper"
)

type SupportController struct {
	supportMapper *mapper.SupportMapper
}

func NewSupportController(supportMapper *mapper.SupportMapper) *SupportController {
	return &SupportController{supportMapper: supportMapper}
}

func (s *SupportController) List() ([]*entity.Support, error) {
	return s.supportMapper.List()
}

func (s *SupportController) Get() (*entity.Support, error) {
	return s.supportMapper.Get()
}

func (s *SupportController) Page() (*entity.Support, error) {
	return s.supportMapper.Page()
}

func (s *SupportController) Create(e *dto.SupportDto) (*entity.Support, error) {
	return s.supportMapper.Create(e)
}
