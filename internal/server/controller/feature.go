package controller

import (
	"context"
	"errors"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
)

// FeatureController manages feature flag read and write operations.
type FeatureController interface {
	ListFlags(ctx context.Context) ([]dto.FeatureFlagItem, error)
	UpdateFlag(ctx context.Context, key string, enabled bool) error
}

type featureController struct {
	store store.Store
}

// NewFeatureController creates a new FeatureController.
func NewFeatureController(st store.Store) FeatureController {
	return &featureController{store: st}
}

func (c *featureController) ListFlags(ctx context.Context) ([]dto.FeatureFlagItem, error) {
	dbValues, err := c.store.SystemConfig().GetAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]dto.FeatureFlagItem, 0, len(models.FeatureFlagDefs))
	for _, def := range models.FeatureFlagDefs {
		enabled := def.Default
		if v, ok := dbValues[def.Key]; ok {
			enabled = v == "true"
		}
		result = append(result, dto.FeatureFlagItem{
			Key:     def.Key,
			Label:   def.Label,
			Group:   def.Group,
			Enabled: enabled,
		})
	}
	return result, nil
}

func (c *featureController) UpdateFlag(ctx context.Context, key string, enabled bool) error {
	// Validate key is a known feature flag.
	valid := false
	for _, def := range models.FeatureFlagDefs {
		if def.Key == key {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("unknown feature flag key")
	}

	val := "false"
	if enabled {
		val = "true"
	}
	return c.store.SystemConfig().Set(ctx, key, val)
}
