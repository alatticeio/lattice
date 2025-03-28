package controller

import (
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/pkg/log"
)

type SupportController struct {
	logger         *log.Logger
	supportService service.SupportService
}

func NewSupportController(supportService service.SupportService) *SupportController {
	return &SupportController{supportService: supportService, logger: log.NewLogger(log.Loglevel, "support-controller")}
}

func (s *SupportController) List() ([]*entity.Support, error) {
	return s.supportService.List()
}

func (s *SupportController) Get() (*entity.Support, error) {
	return s.supportService.Get()
}

func (s *SupportController) Page() (*entity.Support, error) {
	return s.supportService.Page()
}

func (s *SupportController) Create(e *dto.SupportDto) (*entity.Support, error) {
	return s.supportService.Create(e)
}
