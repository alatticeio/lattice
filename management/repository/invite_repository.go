package repository

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/management/utils"
	"wireflow/pkg/log"

	"gorm.io/gorm"
)

type InviteRepository interface {
	WithTx(tx *gorm.DB) InviteRepository

	// inviter
	CreateInviter(ctx context.Context, invite *entity.InviterEntity) error
	DeleteInviter(ctx context.Context, id uint64) error
	UpdateInviter(ctx context.Context, invite *entity.InviterEntity) error
	ListInviters(ctx context.Context, params *dto.InviterParams) ([]*entity.InviterEntity, int64, error)

	// invitee
	CreateInvitee(ctx context.Context, invite *entity.InviteeEntity) error
	DeleteInvitee(ctx context.Context, id uint64) error
	GetByName(ctx context.Context, name string) (*entity.InviteeEntity, error)
	GetByInviteeIdEmail(ctx context.Context, inviteeId uint64, email string) (*entity.InviteeEntity, error)
	UpdateInvitee(ctx context.Context, invite *entity.InviteeEntity) error
	ListInvitees(ctx context.Context, params *dto.InvitationParams) ([]*entity.InviteeEntity, int64, error)
}

var (
	_ InviteRepository = (*inviteRepository)(nil)
)

type inviteRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func (r *inviteRepository) GetByName(ctx context.Context, name string) (*entity.InviteeEntity, error) {
	var invitee entity.InviteeEntity
	if err := r.db.WithContext(ctx).Model(&entity.InviteeEntity{}).Where("invitee_name = ?", name).Find(&invitee).Error; err != nil {
		return nil, err
	}

	return &invitee, nil
}

func (r *inviteRepository) ListInvitees(ctx context.Context, params *dto.InvitationParams) ([]*entity.InviteeEntity, int64, error) {
	var (
		err             error
		inviteeEntities []*entity.InviteeEntity
		count           int64
	)
	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.InviteeEntity{}))
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err := query.Find(&inviteeEntities).Error; err != nil {
		return nil, 0, err
	}
	return inviteeEntities, count, err
}

func (r *inviteRepository) ListInviters(ctx context.Context, params *dto.InviterParams) ([]*entity.InviterEntity, int64, error) {
	var (
		err             error
		inviterEntities []*entity.InviterEntity
		count           int64
	)
	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.InviterEntity{}))
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err := query.Find(&inviterEntities).Error; err != nil {
		return nil, 0, err
	}
	return inviterEntities, count, err
}

func (r *inviteRepository) GetByInviteeIdEmail(ctx context.Context, inviteeId uint64, email string) (*entity.InviteeEntity, error) {
	var invitee entity.InviteeEntity
	if err := r.db.WithContext(ctx).Model(&entity.InviteeEntity{}).Where("invitee_id = ? and email = ?", inviteeId, email).Find(&invitee).Error; err != nil {
		return nil, err
	}

	return &invitee, nil
}

func NewInviteRepository(db *gorm.DB) InviteRepository {
	return &inviteRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "invite-repository"),
	}
}

func (r *inviteRepository) WithTx(tx *gorm.DB) InviteRepository {
	return NewInviteRepository(tx)
}

func (i *inviteRepository) CreateInvitee(ctx context.Context, invite *entity.InviteeEntity) error {
	return i.db.WithContext(ctx).Create(invite).Error
}

func (i *inviteRepository) DeleteInvitee(ctx context.Context, id uint64) error {
	return i.db.WithContext(ctx).Model(&entity.InviteeEntity{}).Where("id = ?", id).Delete(&entity.InviteeEntity{}).Error
}

func (i *inviteRepository) UpdateInvitee(ctx context.Context, invite *entity.InviteeEntity) error {
	return i.db.WithContext(ctx).Updates(invite).Error
}

func (r *inviteRepository) DeleteInviter(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&entity.InviterEntity{}).Where("id = ?", id).Delete(&entity.InviterEntity{}).Error
}

func (r *inviteRepository) UpdateInviter(ctx context.Context, invite *entity.InviterEntity) error {
	return r.db.WithContext(ctx).Updates(invite).Error
}

func (r *inviteRepository) CreateInviter(ctx context.Context, invite *entity.InviterEntity) error {
	return r.db.WithContext(ctx).Create(invite).Error
}
