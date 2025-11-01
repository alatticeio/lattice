package repository

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/pkg/log"
	"wireflow/pkg/utils"

	"gorm.io/gorm"
)

type GroupMemberRepository interface {
	WithTx(tx *gorm.DB) GroupMemberRepository
	Create(ctx context.Context, groupMember *entity.GroupMember) error
	Delete(ctx context.Context, groupMemberId uint64) error
	Update(ctx context.Context, dto *dto.GroupMemberDto) error
	Find(ctx context.Context, groupMemberId uint64) (*entity.Node, error)

	List(ctx context.Context, params *dto.GroupMemberParams) ([]*entity.GroupMember, int64, error)
	QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, error)
}

var (
	_ GroupMemberRepository = (*groupMemberRepository)(nil)
)

type groupMemberRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewGroupMemberRepository(db *gorm.DB) GroupMemberRepository {
	return &groupMemberRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "group-member-repository"),
	}
}

func (r *groupMemberRepository) WithTx(tx *gorm.DB) GroupMemberRepository {
	return NewGroupMemberRepository(tx)
}

func (r *groupMemberRepository) Create(ctx context.Context, groupMember *entity.GroupMember) error {
	return r.db.WithContext(ctx).Create(groupMember).Error
}

func (r *groupMemberRepository) Delete(ctx context.Context, groupMemberId uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.Node{}, groupMemberId).Error
}

func (r *groupMemberRepository) Update(ctx context.Context, dto *dto.GroupMemberDto) error {
	member := entity.GroupMember{
		Role:      dto.Role,
		Status:    dto.Status,
		UpdatedBy: dto.UpdatedBy,
	}
	return r.db.WithContext(ctx).Model(&entity.GroupMember{}).Where("id = ?", dto.ID).Updates(&member).Error
}

func (r *groupMemberRepository) Find(ctx context.Context, nodeId uint64) (*entity.Node, error) {
	var node entity.Node
	err := r.db.WithContext(ctx).First(&node, nodeId).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *groupMemberRepository) List(ctx context.Context, params *dto.GroupMemberParams) ([]*entity.GroupMember, int64, error) {
	var (
		groupMembers []*entity.GroupMember
		count        int64
		err          error
	)

	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.GroupMember{}))
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if params.Page != nil {
		query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err := query.Find(&groupMembers).Error; err != nil {
		return nil, 0, err
	}

	return groupMembers, count, nil
}

func (r *groupMemberRepository) QueryNodes(ctx context.Context, params *dto.QueryParams) ([]*entity.Node, error) {
	var nodes []*entity.Node
	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.Node{}))
	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}

	return nodes, nil
}

func (r *groupMemberRepository) GetAddress() int64 {
	var count int64
	if err := r.db.Model(&entity.Node{}).Count(&count).Error; err != nil {
		r.logger.Errorf("err： %s", err.Error())
		return -1
	}
	if count > 253 {
		return -1
	}
	return count
}
