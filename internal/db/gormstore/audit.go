package gormstore

import (
	"context"
	"time"

	"wireflow/internal/store"
	"wireflow/management/models"

	"gorm.io/gorm"
)

type auditLogRepo struct {
	db *gorm.DB
}

func newAuditLogRepo(db *gorm.DB) *auditLogRepo {
	return &auditLogRepo{db: db}
}

func (r *auditLogRepo) Create(ctx context.Context, log *models.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *auditLogRepo) BatchCreate(ctx context.Context, logs []*models.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(logs, 100).Error
}

func (r *auditLogRepo) List(ctx context.Context, f store.AuditLogFilter) ([]*models.AuditLog, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.AuditLog{})

	if f.WorkspaceID != "" {
		q = q.Where("workspace_id = ?", f.WorkspaceID)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.Resource != "" {
		q = q.Where("resource = ?", f.Resource)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		q = q.Where("user_name LIKE ? OR resource_name LIKE ?", like, like)
	}
	if f.From != "" {
		if t, err := time.Parse(time.RFC3339, f.From); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if f.To != "" {
		if t, err := time.Parse(time.RFC3339, f.To); err == nil {
			q = q.Where("created_at <= ?", t)
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	var logs []*models.AuditLog
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error
	return logs, total, err
}
