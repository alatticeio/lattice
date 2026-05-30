package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/google/uuid"
)

// AgentIdentityService handles AgentIdentity CRUD operations.
type AgentIdentityService struct {
	store  store.Store
	logger *log.Logger
}

// NewAgentIdentityService creates a new AgentIdentityService.
func NewAgentIdentityService(st store.Store) *AgentIdentityService {
	return &AgentIdentityService{store: st, logger: log.GetLogger("agent-identity-service")}
}

// CreateAgentIdentityRequest is the request body for creating/updating an AgentIdentity.
type CreateAgentIdentityRequest struct {
	Name              string   `json:"name" binding:"required"`
	PeerRef           string   `json:"peer_ref" binding:"required"`
	AllowedTools      []string `json:"allowed_tools,omitempty"`
	AllowedNamespaces []string `json:"allowed_namespaces,omitempty"`
	Sandbox           string   `json:"sandbox,omitempty"`
	AuditLevel        string   `json:"audit_level,omitempty"`
	EnforcementMode   string   `json:"enforcement_mode,omitempty"`
	ParentRef         string   `json:"parent_ref,omitempty"`
	SpawnableRoles    []string `json:"spawnable_roles,omitempty"`
	Description       string   `json:"description,omitempty"`
}

// Create creates a new AgentIdentity.
func (s *AgentIdentityService) Create(ctx context.Context, tenantID string, req CreateAgentIdentityRequest) (*models.AgentIdentity, error) {
	toolsJSON, _ := json.Marshal(req.AllowedTools)
	nsJSON, _ := json.Marshal(req.AllowedNamespaces)
	rolesJSON, _ := json.Marshal(req.SpawnableRoles)

	m := &models.AgentIdentity{
		Model:             models.Model{ID: uuid.New().String()},
		TenantID:          tenantID,
		Name:              req.Name,
		PeerRef:           req.PeerRef,
		AllowedTools:      string(toolsJSON),
		AllowedNamespaces: string(nsJSON),
		Sandbox:           req.Sandbox,
		AuditLevel:        req.AuditLevel,
		EnforcementMode:   req.EnforcementMode,
		ParentRef:         req.ParentRef,
		SpawnableRoles:    string(rolesJSON),
		Phase:             "Pending",
		Description:       req.Description,
	}
	if m.Sandbox == "" {
		m.Sandbox = "none"
	}
	if m.AuditLevel == "" {
		m.AuditLevel = "write"
	}
	if m.EnforcementMode == "" {
		m.EnforcementMode = "enforce"
	}
	if err := s.store.AgentIdentities().Create(ctx, m); err != nil {
		return nil, fmt.Errorf("create agent identity: %w", err)
	}
	return m, nil
}

// List lists all AgentIdentities for a tenant.
func (s *AgentIdentityService) List(ctx context.Context, tenantID string) ([]*models.AgentIdentity, error) {
	return s.store.AgentIdentities().ListByTenant(ctx, tenantID)
}

// Get gets a single AgentIdentity by ID.
func (s *AgentIdentityService) Get(ctx context.Context, id string) (*models.AgentIdentity, error) {
	return s.store.AgentIdentities().GetByID(ctx, id)
}

// Update updates an existing AgentIdentity.
func (s *AgentIdentityService) Update(ctx context.Context, id string, req CreateAgentIdentityRequest) (*models.AgentIdentity, error) {
	m, err := s.store.AgentIdentities().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	toolsJSON, _ := json.Marshal(req.AllowedTools)
	nsJSON, _ := json.Marshal(req.AllowedNamespaces)
	rolesJSON, _ := json.Marshal(req.SpawnableRoles)

	m.Name = req.Name
	m.PeerRef = req.PeerRef
	m.AllowedTools = string(toolsJSON)
	m.AllowedNamespaces = string(nsJSON)
	if req.Sandbox != "" {
		m.Sandbox = req.Sandbox
	}
	if req.AuditLevel != "" {
		m.AuditLevel = req.AuditLevel
	}
	if req.EnforcementMode != "" {
		m.EnforcementMode = req.EnforcementMode
	}
	m.ParentRef = req.ParentRef
	m.SpawnableRoles = string(rolesJSON)
	m.Description = req.Description
	if err := s.store.AgentIdentities().Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Delete deletes an AgentIdentity by ID.
func (s *AgentIdentityService) Delete(ctx context.Context, id string) error {
	return s.store.AgentIdentities().Delete(ctx, id)
}
