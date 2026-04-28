package controller

import (
	"context"
	"time"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/resource"
	"wireflow/management/service"
	"wireflow/management/vo"
)

type WorkspaceController interface {
	AddWorkspace(ctx context.Context, workspaceDto *dto.WorkspaceDto) (*vo.WorkspaceVo, error)
	UpdateWorkspace(ctx context.Context, id string, workspaceDto *dto.WorkspaceDto) (*vo.WorkspaceVo, error)
	DeleteWorkspace(ctx context.Context, id string) error
	ListWorkspaces(ctx context.Context, request *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error)
}

type WorkspaceMemberController interface {
	Add(ctx context.Context, workspaceID, userID string, role dto.WorkspaceRole) error
	List(ctx context.Context, workspaceID string) ([]*vo.MemberVo, error)
	UpdateRole(ctx context.Context, workspaceID, userID string, role dto.WorkspaceRole) error
	Remove(ctx context.Context, workspaceID, userID string) error
}

type workspaceController struct {
	workspaceService service.WorkspaceService
}

func (c *workspaceController) DeleteWorkspace(ctx context.Context, id string) error {
	return c.workspaceService.DeleteWorkspace(ctx, id)
}

func (c *workspaceController) ListWorkspaces(ctx context.Context, request *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error) {
	return c.workspaceService.ListWorkspaces(ctx, request)
}

func (c *workspaceController) AddWorkspace(ctx context.Context, workspaceDto *dto.WorkspaceDto) (*vo.WorkspaceVo, error) {
	return c.workspaceService.AddWorkspace(ctx, workspaceDto)
}

func (c *workspaceController) UpdateWorkspace(ctx context.Context, id string, workspaceDto *dto.WorkspaceDto) (*vo.WorkspaceVo, error) {
	return c.workspaceService.UpdateWorkspace(ctx, id, workspaceDto)
}

type workspaceMemberController struct {
	svc service.WorkspaceMemberService
}

func (c *workspaceMemberController) Add(ctx context.Context, workspaceID, userID string, role dto.WorkspaceRole) error {
	now := time.Now()
	_, err := c.svc.Create(ctx, &models.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
		Status:      "active",
		JoinedAt:    &now,
	})
	return err
}

func (c *workspaceMemberController) List(ctx context.Context, workspaceID string) ([]*vo.MemberVo, error) {
	members, err := c.svc.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	vos := make([]*vo.MemberVo, 0, len(members))
	for _, m := range members {
		provider := "local"
		if len(m.User.Identities) > 0 {
			provider = m.User.Identities[0].Provider
		}
		joinedAt := ""
		if m.JoinedAt != nil {
			joinedAt = m.JoinedAt.Format("2006-01-02T15:04:05Z")
		}
		vos = append(vos, &vo.MemberVo{
			UserID:   m.UserID,
			Name:     m.User.Username,
			Email:    m.User.Email,
			Avatar:   m.User.Avatar,
			Role:     m.Role,
			Provider: provider,
			Status:   m.Status,
			JoinedAt: joinedAt,
		})
	}
	return vos, nil
}

func (c *workspaceMemberController) UpdateRole(ctx context.Context, workspaceID, userID string, role dto.WorkspaceRole) error {
	_, err := c.svc.Update(ctx, &models.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
	})
	return err
}

func (c *workspaceMemberController) Remove(ctx context.Context, workspaceID, userID string) error {
	return c.svc.Delete(ctx, &models.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
}

func NewWorkspaceMemberController(st store.Store) WorkspaceMemberController {
	return &workspaceMemberController{svc: service.NewWorkspaceMemberService(st)}
}

func NewWorkspaceController(client *resource.Client, st store.Store) WorkspaceController {
	return &workspaceController{
		workspaceService: service.NewWorkspaceService(client, st),
	}
}
