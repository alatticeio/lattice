package service

import (
	"linkany/management/dto"
	"linkany/management/entity"
)

type SupportService interface {
	// List returns a list of supports
	List() ([]*entity.Support, error)
	Get() (*entity.Support, error)
	Page() (*entity.Support, error)
	Create(e *dto.SupportDto) (*entity.Support, error)
}

var (
	_ SupportService = (*supportServiceImpl)(nil)
)

type supportServiceImpl struct {
	*DatabaseService
}

func (s supportServiceImpl) List() ([]*entity.Support, error) {
	//TODO implement me
	panic("implement me")
}

func (s supportServiceImpl) Get() (*entity.Support, error) {
	//TODO implement me
	panic("implement me")
}

func (s supportServiceImpl) Page() (*entity.Support, error) {
	//TODO implement me
	panic("implement me")
}

func (s supportServiceImpl) Create(e *dto.SupportDto) (*entity.Support, error) {
	//TODO implement me
	panic("implement me")
}

func NewSupportMapper(db *DatabaseService) *supportServiceImpl {
	return &supportServiceImpl{DatabaseService: db}
}
