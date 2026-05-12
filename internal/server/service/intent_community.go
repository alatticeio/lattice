//go:build !pro

package service

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/llm"
	"github.com/alatticeio/lattice/internal/server/resource"
)

type intentServiceStub struct{}

// NewIntentService creates a Community-edition stub — all methods return 402.
func NewIntentService(_ llm.Client, _ *resource.Client, _ store.Store) IntentService {
	return &intentServiceStub{}
}

func (s *intentServiceStub) Plan(_ context.Context, _ IntentRequest) (*IntentPlanView, error) {
	return nil, ErrPaymentRequired("network intent engine is a Pro feature")
}

func (s *intentServiceStub) Apply(_ context.Context, _, _ string) ([]string, error) {
	return nil, ErrPaymentRequired("network intent engine is a Pro feature")
}

func (s *intentServiceStub) History(_ context.Context, _ string, _ int) ([]*IntentHistoryItem, error) {
	return nil, ErrPaymentRequired("network intent engine is a Pro feature")
}
