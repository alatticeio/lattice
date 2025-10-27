package service

import (
	"wireflow/management/dto"
	"wireflow/management/entity"
)

type SupportService interface {
	// List returns a list of supports
	List() ([]*entity.Support, error)
	Get() (*entity.Support, error)
	Page() (*entity.Support, error)
	Create(e *dto.SupportDto) (*entity.Support, error)
}
