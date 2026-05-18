// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gormstore

import (
	"context"
	"time"

	"github.com/alatticeio/lattice/internal/server/models"
	"gorm.io/gorm"
)

type toolSpanRepo struct{ db *gorm.DB }

// NewToolSpanRepo returns a new ToolSpan repository.
func NewToolSpanRepo(db *gorm.DB) *toolSpanRepo {
	return &toolSpanRepo{db: db}
}

func (r *toolSpanRepo) Write(ctx context.Context, span *models.ToolSpan) error {
	return r.db.WithContext(ctx).Create(span).Error
}

func (r *toolSpanRepo) Get(ctx context.Context, traceID string) (*models.ToolSpan, error) {
	var s models.ToolSpan
	if err := r.db.WithContext(ctx).Where("trace_id = ?", traceID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *toolSpanRepo) List(ctx context.Context, agentID string, from, to time.Time, limit int) ([]*models.ToolSpan, error) {
	var spans []*models.ToolSpan
	q := r.db.WithContext(ctx).Order("started_at desc")
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	if !from.IsZero() {
		q = q.Where("started_at >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("started_at <= ?", to)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	return spans, q.Find(&spans).Error
}
