package service

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/management/repository"
	"wireflow/pkg/log"
)

type NodeToService interface {
	FindNodeToList(ctx context.Context, dto *dto.NodeToDto) ([]entity.NodeTo, error)
	AddNodeTo(ctx context.Context, dto *dto.NodeToDto) error
	DeleteNodeToByNodeToId(ctx context.Context, nodeToId uint64) error
	UpdateNodeTo(ctx context.Context) error
}

type nodeToServiceImpl struct {
	logger     *log.Logger
	nodeToRepo repository.NodeToRepository
}

func (n nodeToServiceImpl) FindNodeToList(ctx context.Context, dto *dto.NodeToDto) ([]entity.NodeTo, error) {
	return nil, nil
}

func (n nodeToServiceImpl) AddNodeTo(ctx context.Context, dto *dto.NodeToDto) error {
	//TODO implement me
	panic("implement me")
}

func (n nodeToServiceImpl) DeleteNodeToByNodeToId(ctx context.Context, nodeToId uint64) error {
	return n.nodeToRepo.DeleteByNodeToId(ctx, nodeToId)
}

func (n nodeToServiceImpl) UpdateNodeTo(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func NewNodeToService() NodeToService {
	return &nodeToServiceImpl{}
}
