package repository

import (
	"context"
	"errors"
	"fmt"
	"wireflow/management/database"
	"wireflow/management/dto"
	"wireflow/management/model"
	"wireflow/pkg/utils"

	"gorm.io/gorm"
)

type WorkspaceRepository interface {
	WithTx(tx *gorm.DB) WorkspaceRepository // 关键：返回接口
	CheckPermission(ctx context.Context, userID, teamID string) (bool, error)
	Create(ctx context.Context, workspace *dto.WorkspaceDto) (*model.Workspace, error)
	Update(ctx context.Context, workspace *dto.WorkspaceDto) (*model.Workspace, error)
	Delete(ctx context.Context, workspace *dto.WorkspaceDto) error

	List(ctx context.Context, request *dto.PageRequest) ([]model.Workspace, int64, error)
	FindByNs(ctx context.Context, namespace string) (model.Workspace, error)

	FindById(ctx context.Context, wsId string) (*model.Workspace, error)
}

type WorkspaceMemberRepository interface {
	Create(ctx context.Context, workspace *model.WorkspaceMember) (*model.WorkspaceMember, error)
	Update(ctx context.Context, workspace *model.WorkspaceMember) (*model.WorkspaceMember, error)
	Delete(ctx context.Context, workspace *model.WorkspaceMember) error
	List(ctx context.Context, request *dto.PageRequest) ([]model.WorkspaceMember, int64, error)

	// GetMemberRole 获取用户在特定工作区中的角色
	GetMemberRole(ctx context.Context, workspaceSlug string, userID string) (dto.WorkspaceRole, error)
}

type workspaceRepository struct {
	db *gorm.DB
}

func (r *workspaceRepository) FindById(ctx context.Context, wsId string) (*model.Workspace, error) {
	var workspace model.Workspace
	if err := r.db.First(&workspace, "id = ?", wsId).Error; err != nil {
		return nil, err
	}

	return &workspace, nil
}

func (r *workspaceRepository) FindByNs(ctx context.Context, namespace string) (model.Workspace, error) {
	var workspace model.Workspace
	err := r.db.Where("namespace = ?", namespace).First(&workspace).Error
	return workspace, err
}

// 实现 WithTx
func (r *workspaceRepository) WithTx(tx *gorm.DB) WorkspaceRepository {
	// 返回一个新的实例，但 db 替换为事务 tx
	return &workspaceRepository{db: tx}
}

func (t *workspaceRepository) Create(ctx context.Context, dto *dto.WorkspaceDto) (*model.Workspace, error) {
	var err error
	newNS := model.Workspace{
		Slug:        utils.GenerateSlug(dto.Slug),
		DisplayName: dto.DisplayName,
		Namespace:   dto.Namespace,
	}

	if err = t.db.WithContext(ctx).Create(&newNS).Error; err != nil {
		return nil, err
	}

	return &newNS, nil
}

func (t *workspaceRepository) Update(ctx context.Context, workspace *dto.WorkspaceDto) (*model.Workspace, error) {
	//TODO implement me
	panic("implement me")
}

func (t *workspaceRepository) Delete(ctx context.Context, workspace *dto.WorkspaceDto) error {
	//TODO implement me
	panic("implement me")
}

func (t *workspaceRepository) List(ctx context.Context, request *dto.PageRequest) ([]model.Workspace, int64, error) {
	var workspaces []model.Workspace
	var total int64

	// 1. 构建初始查询对象（关联 Context）
	query := t.db.WithContext(ctx).Model(&model.Workspace{})

	// 2. 添加过滤条件 (假设 request 中有 Keyword 字段)
	if request.Keyword != "" {
		// 模糊搜索 DisplayName 或 Slug
		query = query.Where("display_name LIKE ? OR slug LIKE ?",
			"%"+request.Keyword+"%", "%"+request.Keyword+"%")
	}

	// 3. 执行 Count（必须在 Offset/Limit 之前执行，否则查的是当前页条数）
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count workspace: %v", err)
	}

	// 4. 执行分页查询
	offset := (request.Page - 1) * request.PageSize
	if err := query.Offset(offset).Limit(request.PageSize).Find(&workspaces).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to query workspace: %v", err)
	}

	return workspaces, total, nil
}

type workspaceMemberRepository struct {
	db *gorm.DB
}

func (r *workspaceMemberRepository) GetMemberRole(ctx context.Context, workspaceSlug string, userID string) (dto.WorkspaceRole, error) {
	var member model.WorkspaceMember
	err := r.db.WithContext(ctx).
		Table("workspace_member").
		Joins("JOIN workspace ON workspaces.id = workspace_members.workspace_id").
		Where("workspace.slug = ? AND workspace_member.user_id = ? AND workspace_members.status = ?", workspaceSlug, userID, "active").
		Select("workspace_members.role").
		First(&member).Error

	return member.Role, err
}

func (r *workspaceMemberRepository) Create(ctx context.Context, workspaceMember *model.WorkspaceMember) (*model.WorkspaceMember, error) {
	err := r.db.WithContext(ctx).Create(workspaceMember).Error
	if err != nil {
		return nil, err
	}

	return workspaceMember, nil
}

func (r *workspaceMemberRepository) Update(ctx context.Context, workspace *model.WorkspaceMember) (*model.WorkspaceMember, error) {
	//TODO implement me
	panic("implement me")
}

func (r *workspaceMemberRepository) Delete(ctx context.Context, workspace *model.WorkspaceMember) error {
	//TODO implement me
	panic("implement me")
}

func (r *workspaceMemberRepository) List(ctx context.Context, request *dto.PageRequest) ([]model.WorkspaceMember, int64, error) {

	userID, ok := ctx.Value("user_id").(string)
	if !ok {
		return nil, 0, errors.New("unauthorized: user_id not found in context")
	}

	// 2. 从数据库分页查询用户所属的工作区及其角色
	var members []model.WorkspaceMember
	var total int64

	// 基础查询：关联 Workspace 表
	query := r.db.WithContext(ctx).Model(&model.WorkspaceMember{}).
		Preload("Workspace").
		Where("user_id = ?", userID)

	// 执行总数统计
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count workspaces: %v", err)
	}

	// 执行分页查询
	if err := query.Offset((request.Page - 1) * request.PageSize).
		Limit(request.PageSize).
		Find(&members).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to query workspace members: %v", err)
	}

	return members, total, nil
}

func (t *workspaceRepository) CheckPermission(ctx context.Context, userID, teamID string) (bool, error) {
	// 3. 数据库查询：校验 WorkspaceMember 关系
	var member model.WorkspaceMember
	err := t.db.Where("user_id = ? AND team_id = ? AND status = ?", userID, teamID, "active").First(&member).Error

	if err != nil {
		return false, err
	}

	return member.Status == "active", nil
}

func NewWorkspaceRepository() WorkspaceRepository {
	return &workspaceRepository{db: database.DB}
}

func NewWorkspaceMemberRepository() WorkspaceMemberRepository {
	return &workspaceMemberRepository{db: database.DB}
}
