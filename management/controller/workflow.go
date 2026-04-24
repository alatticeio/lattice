package controller

import (
	"context"

	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/service"
	"wireflow/management/vo"
)

// WorkflowController handles workflow approval request operations.
type WorkflowController interface {
	Submit(ctx context.Context, req service.SubmitWorkflowReq) (*vo.WorkflowRequestVo, error)
	Approve(ctx context.Context, id, reviewerID, reviewerName, note string) error
	Reject(ctx context.Context, id, reviewerID, reviewerName, note string) error
	List(ctx context.Context, filter store.WorkflowFilter) (*dto.PageResult[vo.WorkflowRequestVo], error)
	GetByID(ctx context.Context, id string) (*vo.WorkflowRequestVo, error)
}

type workflowController struct {
	svc service.WorkflowService
}

func NewWorkflowController(svc service.WorkflowService) WorkflowController {
	return &workflowController{svc: svc}
}

func (c *workflowController) Submit(ctx context.Context, req service.SubmitWorkflowReq) (*vo.WorkflowRequestVo, error) {
	wr, err := c.svc.Submit(ctx, req)
	if err != nil {
		return nil, err
	}
	v := toWorkflowVo(wr)
	return &v, nil
}

func (c *workflowController) Approve(ctx context.Context, id, reviewerID, reviewerName, note string) error {
	return c.svc.Approve(ctx, id, reviewerID, reviewerName, note)
}

func (c *workflowController) Reject(ctx context.Context, id, reviewerID, reviewerName, note string) error {
	return c.svc.Reject(ctx, id, reviewerID, reviewerName, note)
}

func (c *workflowController) List(ctx context.Context, filter store.WorkflowFilter) (*dto.PageResult[vo.WorkflowRequestVo], error) {
	list, total, err := c.svc.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	vos := make([]vo.WorkflowRequestVo, 0, len(list))
	for _, wr := range list {
		vos = append(vos, toWorkflowVo(wr))
	}
	return &dto.PageResult[vo.WorkflowRequestVo]{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
		List:     vos,
	}, nil
}

func (c *workflowController) GetByID(ctx context.Context, id string) (*vo.WorkflowRequestVo, error) {
	wr, err := c.svc.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	v := toWorkflowVo(wr)
	return &v, nil
}

func toWorkflowVo(wr *models.WorkflowRequest) vo.WorkflowRequestVo {
	v := vo.WorkflowRequestVo{
		ID:               wr.ID,
		CreatedAt:        wr.CreatedAt.Format("2006-01-02T15:04:05Z"),
		WorkspaceID:      wr.WorkspaceID,
		RequestedBy:      wr.RequestedBy,
		RequestedByName:  wr.RequestedByName,
		RequestedByEmail: wr.RequestedByEmail,
		ResourceType:     wr.ResourceType,
		ResourceName:     wr.ResourceName,
		Action:           wr.Action,
		Status:           string(wr.Status),
		ReviewedBy:       wr.ReviewedBy,
		ReviewedByName:   wr.ReviewedByName,
		ReviewNote:       wr.ReviewNote,
		ErrorMessage:     wr.ErrorMessage,
	}
	if wr.ReviewedAt != nil {
		s := wr.ReviewedAt.Format("2006-01-02T15:04:05Z")
		v.ReviewedAt = &s
	}
	if wr.ExecutedAt != nil {
		s := wr.ExecutedAt.Format("2006-01-02T15:04:05Z")
		v.ExecutedAt = &s
	}
	return v
}
