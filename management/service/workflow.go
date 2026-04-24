package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/models"

	"github.com/google/uuid"
)

// ExecutorFunc performs the actual operation encoded in a WorkflowRequest payload.
// It receives the raw JSON payload and returns an error if execution failed.
type ExecutorFunc func(ctx context.Context, payload string) error

// WorkflowService manages approval workflow requests.
type WorkflowService interface {
	// Submit creates a new pending workflow request. Returns the saved request.
	Submit(ctx context.Context, req SubmitWorkflowReq) (*models.WorkflowRequest, error)
	// Approve moves a request from pending → approved and schedules execution.
	Approve(ctx context.Context, id, reviewerID, reviewerName, note string) error
	// Reject moves a request from pending → rejected.
	Reject(ctx context.Context, id, reviewerID, reviewerName, note string) error
	// List returns paginated workflow requests.
	List(ctx context.Context, filter store.WorkflowFilter) ([]*models.WorkflowRequest, int64, error)
	// GetByID returns a single workflow request.
	GetByID(ctx context.Context, id string) (*models.WorkflowRequest, error)

	// RegisterExecutor registers an executor for a (resourceType, action) pair.
	// Must be called at server startup before any requests arrive.
	RegisterExecutor(resourceType, action string, fn ExecutorFunc)
}

// SubmitWorkflowReq carries the data needed to create a workflow request.
type SubmitWorkflowReq struct {
	WorkspaceID      string
	RequestedBy      string
	RequestedByName  string
	RequestedByEmail string
	ResourceType     string
	ResourceName     string
	Action           string
	Payload          string // raw JSON
}

type workflowService struct {
	store     store.Store
	log       *log.Logger
	executors map[string]ExecutorFunc // key: "resourceType:action"
}

func NewWorkflowService(st store.Store) WorkflowService {
	return &workflowService{
		store:     st,
		log:       log.GetLogger("workflow"),
		executors: make(map[string]ExecutorFunc),
	}
}

func (s *workflowService) RegisterExecutor(resourceType, action string, fn ExecutorFunc) {
	key := executorKey(resourceType, action)
	s.executors[key] = fn
	s.log.Info("workflow executor registered", "key", key)
}

func (s *workflowService) Submit(ctx context.Context, req SubmitWorkflowReq) (*models.WorkflowRequest, error) {
	wr := &models.WorkflowRequest{
		ID:               uuid.New().String(),
		WorkspaceID:      req.WorkspaceID,
		RequestedBy:      req.RequestedBy,
		RequestedByName:  req.RequestedByName,
		RequestedByEmail: req.RequestedByEmail,
		ResourceType:     req.ResourceType,
		ResourceName:     req.ResourceName,
		Action:           req.Action,
		Payload:          req.Payload,
		Status:           models.WorkflowStatusPending,
	}
	if err := s.store.WorkflowRequests().Create(ctx, wr); err != nil {
		return nil, fmt.Errorf("create workflow request: %w", err)
	}
	s.log.Info("workflow request submitted", "id", wr.ID, "resource", wr.ResourceType, "action", wr.Action)
	return wr, nil
}

func (s *workflowService) Approve(ctx context.Context, id, reviewerID, reviewerName, note string) error {
	wr, err := s.store.WorkflowRequests().GetByID(ctx, id)
	if err != nil {
		return errors.New("workflow request not found")
	}
	if wr.Status != models.WorkflowStatusPending {
		return fmt.Errorf("cannot approve a request with status %q", wr.Status)
	}

	now := time.Now()
	if err := s.store.WorkflowRequests().UpdateStatus(ctx, id, models.WorkflowStatusApproved, map[string]interface{}{
		"reviewed_by":      reviewerID,
		"reviewed_by_name": reviewerName,
		"reviewed_at":      now,
		"review_note":      note,
	}); err != nil {
		return err
	}

	// Reload to get latest snapshot before execution.
	wr.Status = models.WorkflowStatusApproved
	wr.ReviewedBy = reviewerID
	wr.ReviewedByName = reviewerName
	wr.ReviewedAt = &now
	wr.ReviewNote = note

	go s.execute(context.Background(), wr)
	return nil
}

func (s *workflowService) Reject(ctx context.Context, id, reviewerID, reviewerName, note string) error {
	wr, err := s.store.WorkflowRequests().GetByID(ctx, id)
	if err != nil {
		return errors.New("workflow request not found")
	}
	if wr.Status != models.WorkflowStatusPending {
		return fmt.Errorf("cannot reject a request with status %q", wr.Status)
	}

	now := time.Now()
	return s.store.WorkflowRequests().UpdateStatus(ctx, id, models.WorkflowStatusRejected, map[string]interface{}{
		"reviewed_by":      reviewerID,
		"reviewed_by_name": reviewerName,
		"reviewed_at":      now,
		"review_note":      note,
	})
}

func (s *workflowService) List(ctx context.Context, filter store.WorkflowFilter) ([]*models.WorkflowRequest, int64, error) {
	return s.store.WorkflowRequests().List(ctx, filter)
}

func (s *workflowService) GetByID(ctx context.Context, id string) (*models.WorkflowRequest, error) {
	return s.store.WorkflowRequests().GetByID(ctx, id)
}

// execute runs the registered executor for the given request.
// Called in a separate goroutine after approval.
func (s *workflowService) execute(ctx context.Context, wr *models.WorkflowRequest) {
	key := executorKey(wr.ResourceType, wr.Action)
	fn, ok := s.executors[key]
	if !ok {
		s.log.Error("no executor registered", fmt.Errorf("missing executor for %q", key))
		now := time.Now()
		_ = s.store.WorkflowRequests().UpdateStatus(ctx, wr.ID, models.WorkflowStatusFailed, map[string]interface{}{
			"executed_at":   now,
			"error_message": fmt.Sprintf("no executor registered for %q", key),
		})
		return
	}

	s.log.Info("executing workflow request", "id", wr.ID, "key", key)
	now := time.Now()
	if err := fn(ctx, wr.Payload); err != nil {
		s.log.Error("workflow execution failed", err, "id", wr.ID)
		_ = s.store.WorkflowRequests().UpdateStatus(ctx, wr.ID, models.WorkflowStatusFailed, map[string]interface{}{
			"executed_at":   now,
			"error_message": err.Error(),
		})
		return
	}

	_ = s.store.WorkflowRequests().UpdateStatus(ctx, wr.ID, models.WorkflowStatusExecuted, map[string]interface{}{
		"executed_at": now,
	})
	s.log.Info("workflow request executed", "id", wr.ID)
}

func executorKey(resourceType, action string) string {
	return resourceType + ":" + action
}

