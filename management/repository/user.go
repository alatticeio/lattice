package repository

import (
	"context"
	"wireflow/internal/log"
	"wireflow/management/dto"
	"wireflow/management/model"
	"wireflow/management/vo"

	"gorm.io/gorm"
)

type UserRepository struct {
	log *log.Logger
	*BaseRepository[model.User]
}

func (r *UserRepository) ListWithQuery(ctx context.Context, options *dto.QueryOption) ([]*model.User, error) {
	db := r.db.WithContext(ctx).Model(&model.User{})
	if options != nil {
		if options.UserID != "" {
			db = db.Where("user_id = ?", options.UserID)
		}

		if options.WorkspaceID != "" {
			db = db.Where("workspace_id = ?", options.WorkspaceID)
		}
	}

	var users []*model.User
	if err := db.Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func (r *UserRepository) Login(ctx context.Context, username, password string) (*model.User, error) {

	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "username = ? AND password = ?", username, password).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) OnboardExternalUser(ctx context.Context, subject string, email string) (*model.User, error) {

	user := &model.User{
		Email:      email,
		ExternalID: subject,
	}

	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error) {
	var users []model.User
	var total int64
	var userVos []vo.UserVo

	// 1. 初始化 db 句柄
	query := r.db.WithContext(ctx).Model(&model.User{})

	// 2. 如果有搜索条件（例如按用户名搜索）
	if req.Keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}

	// 3. 统计总数（注意：Count 必须在 Limit/Offset 之前执行）
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 4. 执行分页与关联预加载
	// 假设你想在用户列表里展示他们所属的 Workspaces
	err := query.Debug().
		Preload("Workspaces").
		Limit(req.PageSize).
		Offset((req.Page - 1) * req.PageSize).
		Order("created_at DESC").
		Find(&users).Error

	if err != nil {
		return nil, err
	}

	// 5. 转换为 VO (Value Object)
	// 实际项目中建议使用 copier 库或手动映射
	for _, user := range users {
		userVo := vo.UserVo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Avatar:   user.Avatar,
			Role:     string(user.Role),
			// 可以在这里提取所属 Workspace 的名称列表
		}

		var workspacesVos []vo.WorkspaceVo
		for _, workspace := range user.Workspaces {
			workspacesVos = append(workspacesVos, vo.WorkspaceVo{
				ID:          workspace.ID,
				Slug:        workspace.Slug,
				DisplayName: workspace.DisplayName,
			})

			userVo.Workspaces = append(userVo.Workspaces, workspacesVos...)
		}

		userVos = append(userVos, userVo)

	}

	// 6. 返回标准分页结果
	return &dto.PageResult[vo.UserVo]{
		List:     userVos,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		log:            log.GetLogger("user-repository"),
		BaseRepository: NewBaseRepository[model.User](db),
	}
}
