package repository

import (
	"context"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/pkg/log"

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
		sql         string
		wrappers    []interface{}
		err         error
	)

	//1.base query
	query := r.db.WithContext(ctx).Model(&entity.Permissions{})

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
	if err := query.Find(&permissions).Error; err != nil {
		return nil, 0, err
	}

	return permissions, count, nil
}

func (r *permissionRepository) Query(ctx context.Context, params *dto.PermissionParams) ([]*entity.Permissions, error) {
	var (
		permissions []*entity.Permissions
	)

	sql, wrappers := utils.GenerateLikeSql(params)

	r.logger.Verbosef("sql: %s, wrappers: %v", sql, wrappers)
	if err := r.db.WithContext(ctx).Where(sql, wrappers...).Find(&permissions).Error; err != nil {
		return nil, err
	}

	return permissions, nil
}
