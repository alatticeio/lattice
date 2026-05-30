package service

import (
	"context"
	"fmt"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/google/uuid"
)

// PeerIdentityService handles PeerIdentity CRUD operations.
type PeerIdentityService struct {
	store  store.Store
	logger *log.Logger
}

// NewPeerIdentityService creates a new PeerIdentityService.
func NewPeerIdentityService(st store.Store) *PeerIdentityService {
	return &PeerIdentityService{store: st, logger: log.GetLogger("peer-identity-service")}
}

// CreatePeerIdentityRequest is the request body for creating/updating a PeerIdentity.
type CreatePeerIdentityRequest struct {
	Name               string `json:"name" binding:"required"`
	PeerRef            string `json:"peer_ref" binding:"required"`
	PreviousPeerRef    string `json:"previous_peer_ref,omitempty"`
	GracePeriodSeconds int    `json:"grace_period_seconds,omitempty"`
	Description        string `json:"description,omitempty"`
}

// Create creates a new PeerIdentity.
func (s *PeerIdentityService) Create(ctx context.Context, networkID string, req CreatePeerIdentityRequest) (*models.PeerIdentity, error) {
	m := &models.PeerIdentity{
		Model:              models.Model{ID: uuid.New().String()},
		NetworkID:          networkID,
		Name:               req.Name,
		PeerRef:            req.PeerRef,
		PreviousPeerRef:    req.PreviousPeerRef,
		GracePeriodSeconds: req.GracePeriodSeconds,
		Description:        req.Description,
	}
	if m.GracePeriodSeconds == 0 {
		m.GracePeriodSeconds = 300
	}
	if err := s.store.PeerIdentities().Create(ctx, m); err != nil {
		return nil, fmt.Errorf("create peer identity: %w", err)
	}
	return m, nil
}

// List lists all PeerIdentities for a network.
func (s *PeerIdentityService) List(ctx context.Context, networkID string) ([]*models.PeerIdentity, error) {
	return s.store.PeerIdentities().ListByNetwork(ctx, networkID)
}

// Get gets a single PeerIdentity by ID.
func (s *PeerIdentityService) Get(ctx context.Context, id string) (*models.PeerIdentity, error) {
	return s.store.PeerIdentities().GetByID(ctx, id)
}

// GetByNetworkAndName gets a PeerIdentity by network and name.
func (s *PeerIdentityService) GetByNetworkAndName(ctx context.Context, networkID, name string) (*models.PeerIdentity, error) {
	return s.store.PeerIdentities().GetByNetworkAndName(ctx, networkID, name)
}

// Update updates an existing PeerIdentity.
func (s *PeerIdentityService) Update(ctx context.Context, id string, req CreatePeerIdentityRequest) (*models.PeerIdentity, error) {
	m, err := s.store.PeerIdentities().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Name = req.Name
	m.PeerRef = req.PeerRef
	m.PreviousPeerRef = req.PreviousPeerRef
	if req.GracePeriodSeconds > 0 {
		m.GracePeriodSeconds = req.GracePeriodSeconds
	}
	m.Description = req.Description
	if err := s.store.PeerIdentities().Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Delete deletes a PeerIdentity by ID.
func (s *PeerIdentityService) Delete(ctx context.Context, id string) error {
	return s.store.PeerIdentities().Delete(ctx, id)
}
