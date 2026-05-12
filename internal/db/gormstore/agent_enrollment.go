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

type agentEnrollmentTokenRepo struct{ db *gorm.DB }

// NewAgentEnrollmentTokenRepo returns a new enrollment token repository.
func NewAgentEnrollmentTokenRepo(db *gorm.DB) *agentEnrollmentTokenRepo {
	return &agentEnrollmentTokenRepo{db: db}
}

func (r *agentEnrollmentTokenRepo) Create(ctx context.Context, token *models.AgentEnrollmentToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *agentEnrollmentTokenRepo) GetByToken(ctx context.Context, token string) (*models.AgentEnrollmentToken, error) {
	var t models.AgentEnrollmentToken
	if err := r.db.WithContext(ctx).Where("token = ?", token).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *agentEnrollmentTokenRepo) MarkUsed(ctx context.Context, token string, usedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.AgentEnrollmentToken{}).
		Where("token = ?", token).
		Update("used_at", usedAt).Error
}

func (r *agentEnrollmentTokenRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&models.AgentEnrollmentToken{}).Error
}
