package controller

import (
	"context"
	"wireflow/management/dto"
	"wireflow/management/model"
	"wireflow/management/resource"
	"wireflow/management/service"
	"wireflow/management/vo"
)

type WorkspaceController interface {
	AddWorkspace(ctx context.Context, workspaceDto *dto.WorkspaceDto) (*vo.WorkspaceVo, error)
	ListWorkspaces(ctx context.Context, request *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error)
}

type WorkspaceMemberController interface {
}

type workspaceController struct {
	workspaceService service.WorkspaceService
}

func (c *workspaceController) ListWorkspaces(ctx context.Context, request *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error) {
	return c.workspaceService.ListWorkspaces(ctx, request)
}

func (c *workspaceController) AddWorkspace(ctx context.Context, workspaceDto *dto.WorkspaceDto) (*vo.WorkspaceVo, error) {
	return c.workspaceService.AddWorkspace(ctx, workspaceDto)
}

type workspaceMemberController struct {
	workspaceMemberService service.WorkspaceMemberService
}

func (c workspaceController) OnboardExternalUser(ctx context.Context, userId, extEmail string) (*model.User, error) {
	return c.workspaceService.OnboardExternalUser(ctx, userId, extEmail)
}

func NewWorkspaceController(client *resource.Client) WorkspaceController {
	return &workspaceController{
		workspaceService: service.NewWorkspaceService(client),
	}
}
