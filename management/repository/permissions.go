package repository

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/pkg/log"
	"wireflow/pkg/utils"

	"gorm.io/gorm"
)

type PermissionRepository interface {
	WithTx(tx *gorm.DB) PermissionRepository
	Create(ctx context.Context, accessPolicy *entity.AccessPolicy) error
	Delete(ctx context.Context, accessId uint64) error
	Update(ctx context.Context, accessPolicy *entity.AccessPolicy) error
	Find(ctx context.Context, accessId uint64) (*entity.AccessPolicy, error)
	List(ctx context.Context, params *dto.PermissionParams) ([]*entity.Permissions, int64, error)
	Query(ctx context.Context, params *dto.PermissionParams) ([]*entity.Permissions, error)
}

type UserResourcePermission interface {
	WithTx(tx *gorm.DB) UserResourcePermission
	Create(ctx context.Context, permission *entity.UserResourceGrantedPermission) error
	Delete(ctx context.Context, id uint64) error
	Update(ctx context.Context, permission *entity.UserResourceGrantedPermission) error
	Find(ctx context.Context, id uint64) (*entity.UserResourceGrantedPermission, error)
	List(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.UserResourceGrantedPermission, int64, error)
	Query(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.UserResourceGrantedPermission, error)
}

var (
	_ PermissionRepository = (*permissionRepository)(nil)
)

type permissionRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewPermissionRepository(db *gorm.DB) PermissionRepository {
	return &permissionRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "access-policy-repository"),
	}
}

func (r *permissionRepository) WithTx(tx *gorm.DB) PermissionRepository {
	return NewPermissionRepository(tx)
}

func (r *permissionRepository) Create(ctx context.Context, access *entity.AccessPolicy) error {
	return r.db.WithContext(ctx).Create(access).Error
}

func (r *permissionRepository) Delete(ctx context.Context, accessId uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.Node{}, accessId).Error
}

func (r *permissionRepository) Update(ctx context.Context, accessPolicy *entity.AccessPolicy) error {
	return r.db.WithContext(ctx).Model(&entity.AccessPolicy{}).Where("id = ?", accessPolicy.ID).Updates(accessPolicy).Error
}

func (r *permissionRepository) Find(ctx context.Context, accessId uint64) (*entity.AccessPolicy, error) {
	var access entity.AccessPolicy
	err := r.db.WithContext(ctx).First(&access, accessId).Error
	if err != nil {
		return nil, err
	}
	return &access, nil
}

func (r *permissionRepository) List(ctx context.Context, params *dto.PermissionParams) ([]*entity.Permissions, int64, error) {
	var (
		permissions []*entity.Permissions
		count       int64
		err         error
	)

	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.Permissions{}))
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query = query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err := query.Find(&permissions).Error; err != nil {
		return nil, 0, err
	}

	return permissions, count, nil
}

func (r *permissionRepository) Query(ctx context.Context, params *dto.PermissionParams) ([]*entity.Permissions, error) {
	var (
		permissions []*entity.Permissions
	)

	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.Permissions{}))

	if err := query.Find(&permissions).Error; err != nil {
		return nil, err
	}

	return permissions, nil
}
