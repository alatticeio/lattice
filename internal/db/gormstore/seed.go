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

	"github.com/alatticeio/lattice/internal/server/models"
	"gorm.io/gorm"
)

type seedRepo struct{ db *gorm.DB }

func newSeedRepo(db *gorm.DB) *seedRepo { return &seedRepo{db: db} }

// Clear deletes all seed records belonging to workspaceID in one transaction.
func (r *seedRepo) Clear(ctx context.Context, workspaceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ? AND is_seed = ?", workspaceID, true).
			Delete(&models.AuditLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND is_seed = ?", workspaceID, true).
			Delete(&models.Policy{}).Error; err != nil {
			return err
		}
		return tx.Where("workspace_id = ? AND is_seed = ?", workspaceID, true).
			Delete(&models.AlertHistory{}).Error
	})
}
