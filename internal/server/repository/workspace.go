package repository

import (
	"context"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"

	"gorm.io/gorm"
)

type WorkspaceRepository struct {
	*BaseRepository[models.Workspace]
}

//
//func (t *WorkspaceRepository) List(ctx context.Context, request *dto.PageRequest) ([]model.Workspace, int64, error) {
//	var workspaces []model.Workspace
//	var total int64
//
//	// 1. Build initial query object (with Context)
//	query := t.db.WithContext(ctx).Model(&model.Workspace{})
//
//	// 2. Add filter conditions (assuming request has Keyword field)
//	if request.Keyword != "" {
//		// Fuzzy search DisplayName or Slug
//		query = query.Where("display_name LIKE ? OR slug LIKE ?",
//			"%"+request.Keyword+"%", "%"+request.Keyword+"%")
//	}
//
//	// 3. Execute Count (must be called before Offset/Limit, otherwise it counts the current page)
//	if err := query.Count(&total).Error; err != nil {
//		return nil, 0, fmt.Errorf("failed to count workspace: %v", err)
//	}
//
//	// 4. Execute paginated query
//	offset := (request.Page - 1) * request.PageSize
//	if err := query.Offset(offset).Limit(request.PageSize).Find(&workspaces).Error; err != nil {
//		return nil, 0, fmt.Errorf("failed to query workspace: %v", err)
//	}
//
//	return workspaces, total, nil
//}

type WorkspaceMemberRepository struct {
	*BaseRepository[models.WorkspaceMember]
}

func (r *WorkspaceMemberRepository) GetMemberRole(ctx context.Context, workspaceSlug string, userID string) (dto.WorkspaceRole, error) {
	var member models.WorkspaceMember
	err := r.db.WithContext(ctx).
		Table("workspace_member").
		Joins("JOIN workspace ON workspaces.id = workspace_members.workspace_id").
		Where("workspace.slug = ? AND workspace_member.user_id = ? AND workspace_members.status = ?", workspaceSlug, userID, "active").
		Select("workspace_members.role").
		First(&member).Error

	return member.Role, err
}

//func (r *WorkspaceMemberRepository) List(ctx context.Context, request *dto.PageRequest) ([]model.WorkspaceMember, int64, error) {
//
//	userID, ok := ctx.Value(infra.UserIDKey).(string)
//	if !ok {
//		return nil, 0, errors.New("unauthorized: user_id not found in context")
//	}
//
//	// 2. Paginated query from DB for user's workspaces and their roles
//	var members []model.WorkspaceMember
//	var total int64
//
//	// Base query: join Workspace table
//	query := r.db.WithContext(ctx).Model(&model.WorkspaceMember{}).
//		Preload("Workspace").
//		Where("user_id = ?", userID)
//
//	// Execute total count
//	if err := query.Count(&total).Error; err != nil {
//		return nil, 0, fmt.Errorf("failed to count workspaces: %v", err)
//	}
//
//	// Execute paginated query
//	if err := query.Offset((request.Page - 1) * request.PageSize).
//		Limit(request.PageSize).
//		Find(&members).Error; err != nil {
//		return nil, 0, fmt.Errorf("failed to query workspace members: %v", err)
//	}
//
//	return members, total, nil
//}

func (t *WorkspaceRepository) CheckPermission(ctx context.Context, userID, teamID string) (bool, error) {
	// 3. Database query: validate WorkspaceMember relationship
	var member models.WorkspaceMember
	err := t.db.Where("user_id = ? AND team_id = ? AND status = ?", userID, teamID, "active").First(&member).Error

	if err != nil {
		return false, err
	}

	return member.Status == "active", nil
}

func NewWorkspaceRepository(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{
		BaseRepository: NewBaseRepository[models.Workspace](db)}
}

func NewWorkspaceMemberRepository(db *gorm.DB) *WorkspaceMemberRepository {
	return &WorkspaceMemberRepository{
		BaseRepository: NewBaseRepository[models.WorkspaceMember](db),
	}
}
