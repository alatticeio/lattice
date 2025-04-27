package repository

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

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
	sql, wrappers := utils.Generate(params)
	query := r.db.WithContext(ctx).Model(&entity.UserResourceGrantedPermission{})

	if sql != "" {
		query = query.Where(sql, wrappers)
	}

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
		sql             string
		wrappers        []interface{}
		err             error
	)

	//1.base query
	query := r.db.WithContext(ctx).Model(&entity.UserResourceGrantedPermission{})

	sql, wrappers = utils.Generate(params)
	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)

	//2. add filter params
	query = query.Where(sql, wrappers)

	//3.got total
	if err = query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	//4. add pagination
	if params.Page != nil {
		offset := (*params.Size - 1) * *params.Size
		query = query.Offset(offset).Limit(*params.Size)
	}

	//5. query
	if err := query.Find(&userPermissions).Error; err != nil {
		return nil, 0, err
	}

	return userPermissions, count, nil
}

func (r *userPermissionRepository) Query(ctx context.Context, params *dto.AccessPolicyParams) ([]*entity.UserResourceGrantedPermission, error) {
	var userPermissions []*entity.UserResourceGrantedPermission
	var sql string
	var wrappers []interface{}

	sql, wrappers = utils.Generate(params)

	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := r.db.WithContext(ctx).Where(sql, wrappers...).Find(&userPermissions).Error; err != nil {
		return nil, err
	}

	return userPermissions, nil
}
