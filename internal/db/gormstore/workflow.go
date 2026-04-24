package gormstore

import (
	"context"

	"wireflow/internal/store"
	"wireflow/management/models"

	"gorm.io/gorm"
)

type workflowRepo struct{ db *gorm.DB }

func newWorkflowRepo(db *gorm.DB) store.WorkflowRepository {
	return &workflowRepo{db: db}
}

func (r *workflowRepo) Create(ctx context.Context, req *models.WorkflowRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

func (r *workflowRepo) GetByID(ctx context.Context, id string) (*models.WorkflowRequest, error) {
	var req models.WorkflowRequest
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&req).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *workflowRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.WorkflowRequest{}).Error
}

func (r *workflowRepo) UpdateStatus(ctx context.Context, id string, status models.WorkflowStatus, fields map[string]interface{}) error {
	updates := map[string]interface{}{"status": status}
	for k, v := range fields {
		updates[k] = v
	}
	return r.db.WithContext(ctx).Model(&models.WorkflowRequest{}).
		Where("id = ?", id).Updates(updates).Error
}

func (r *workflowRepo) List(ctx context.Context, filter store.WorkflowFilter) ([]*models.WorkflowRequest, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.WorkflowRequest{})

	if filter.WorkspaceID != "" {
		q = q.Where("workspace_id = ?", filter.WorkspaceID)
	}
	if filter.RequestedBy != "" {
		q = q.Where("requested_by = ?", filter.RequestedBy)
	}
	if filter.ResourceType != "" {
		q = q.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.Action != "" {
		q = q.Where("action = ?", filter.Action)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	var list []*models.WorkflowRequest
	if err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
