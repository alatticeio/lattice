package repository

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/entity"
	"wireflow/management/utils"
	"wireflow/pkg/log"

	"gorm.io/gorm"
)

type UserResourcePermissionRepository interface {
	WithTx(tx *gorm.DB) UserResourcePermissionRepository
	Create(ctx context.Context, permission *entity.UserResourceGrantedPermission) error
	Delete(ctx context.Context, id uint64) error
	DeleteByParams(ctx context.Context, params dto.Params) error
	// Update(ctx context.Context, permission *entity.UserResourceGrantedPermission) error
	Find(ctx context.Context, id uint64) (*entity.UserResourceGrantedPermission, error)
	List(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.UserResourceGrantedPermission, int64, error)
	Query(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.UserResourceGrantedPermission, error)
}

var (
	_ UserResourcePermissionRepository = (*userPermissionRepository)(nil)
)

type userPermissionRepository struct {
	db     *gorm.DB
	logger *log.Logger
}

func (r *userPermissionRepository) DeleteByParams(ctx context.Context, params dto.Params) error {
	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.UserResourceGrantedPermission{}))
	return query.Delete(&entity.UserResourceGrantedPermission{}).Error
}

func NewUserPermissionRepository(db *gorm.DB) UserResourcePermissionRepository {
	return &userPermissionRepository{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "access-policy-repository"),
	}
}

func (r *userPermissionRepository) WithTx(tx *gorm.DB) UserResourcePermissionRepository {
	return NewUserPermissionRepository(tx)
}

func (r *userPermissionRepository) Create(ctx context.Context, userPermission *entity.UserResourceGrantedPermission) error {
	return r.db.WithContext(ctx).Create(userPermission).Error
}

func (r *userPermissionRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&entity.Node{}, id).Error
}

// func (r *userPermissionRepository) Update(ctx context.Context, accessPolicy *entity.AccessPolicy) error {
// 	return r.db.WithContext(ctx).Model(&entity.AccessPolicy{}).Where("id = ?", accessPolicy.ID).Updates(accessPolicy).Error
// }

func (r *userPermissionRepository) Find(ctx context.Context, accessId uint64) (*entity.UserResourceGrantedPermission, error) {
	var userPermission entity.UserResourceGrantedPermission
	err := r.db.WithContext(ctx).First(&userPermission, accessId).Error
	if err != nil {
		return nil, err
	}
	return &userPermission, nil
}

func (r *userPermissionRepository) List(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.UserResourceGrantedPermission, int64, error) {
	var (
		userPermissions []*entity.UserResourceGrantedPermission
		count           int64
		err             error
	)

	conditions := utils.GenerateQuery(params, false)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.UserResourceGrantedPermission{}))
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	pageOffset := params.GetPageOffset()
	if pageOffset != nil {
		query.Offset(pageOffset.Offset).Limit(pageOffset.Limit)
	}

	if err := query.Find(&userPermissions).Error; err != nil {
		return nil, 0, err
	}

	return userPermissions, count, nil
}

func (r *userPermissionRepository) Query(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.UserResourceGrantedPermission, error) {
	var userPermissions []*entity.UserResourceGrantedPermission
	conditions := utils.GenerateQuery(params, true)
	query := conditions.BuildQuery(r.db.WithContext(ctx).Model(&entity.UserResourceGrantedPermission{}))
	if err := query.Find(&userPermissions).Error; err != nil {
		return nil, err
	}

	return userPermissions, nil
}
