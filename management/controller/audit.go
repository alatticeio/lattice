package controller

import (
	"context"

	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/service"
	"wireflow/management/vo"
)

// AuditController handles audit log queries.
type AuditController interface {
	List(ctx context.Context, filter store.AuditLogFilter) (*dto.PageResult[vo.AuditLogVo], error)
}

type auditController struct {
	svc service.AuditService
}

func NewAuditController(svc service.AuditService) AuditController {
	return &auditController{svc: svc}
}

func (c *auditController) List(ctx context.Context, filter store.AuditLogFilter) (*dto.PageResult[vo.AuditLogVo], error) {
	logs, total, err := c.svc.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	vos := make([]vo.AuditLogVo, 0, len(logs))
	for _, l := range logs {
		vos = append(vos, toAuditVo(l))
	}
	return &dto.PageResult[vo.AuditLogVo]{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
		List:     vos,
	}, nil
}

func toAuditVo(l *models.AuditLog) vo.AuditLogVo {
	return vo.AuditLogVo{
		ID:           l.ID,
		CreatedAt:    l.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UserID:       l.UserID,
		UserName:     l.UserName,
		UserEmail:    l.UserEmail,
		UserIP:       l.UserIP,
		WorkspaceID:  l.WorkspaceID,
		Action:       l.Action,
		Resource:     l.Resource,
		ResourceID:   l.ResourceID,
		ResourceName: l.ResourceName,
		Scope:        l.Scope,
		Status:       l.Status,
		StatusCode:   l.StatusCode,
		Detail:       l.Detail,
	}
}
