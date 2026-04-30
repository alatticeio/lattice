package service

import (
	"context"
	"time"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"

	"github.com/google/uuid"
)

// AuditService records and queries audit log entries.
type AuditService interface {
	// Log queues an audit event for async write; never blocks the caller.
	Log(entry models.AuditLog)
	// List returns a paginated, filtered list of audit logs.
	List(ctx context.Context, filter store.AuditLogFilter) ([]*models.AuditLog, int64, error)
	// Start launches the background writer; call once at startup.
	Start(ctx context.Context)
}

type auditService struct {
	store  store.Store
	logger *log.Logger
	ch     chan models.AuditLog
}

func NewAuditService(st store.Store) AuditService {
	return &auditService{
		store:  st,
		logger: log.GetLogger("audit"),
		ch:     make(chan models.AuditLog, 512),
	}
}

func (s *auditService) Log(entry models.AuditLog) {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	// Non-blocking: drop if the buffer is full rather than blocking the HTTP handler.
	select {
	case s.ch <- entry:
	default:
		s.logger.Warn("audit channel full, dropping entry", "action", entry.Action, "resource", entry.Resource)
	}
}

func (s *auditService) List(ctx context.Context, filter store.AuditLogFilter) ([]*models.AuditLog, int64, error) {
	return s.store.AuditLogs().List(ctx, filter)
}

// Start runs the background flush goroutine.
// It batches up to 100 entries or flushes every second, whichever comes first.
func (s *auditService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		buf := make([]*models.AuditLog, 0, 100)

		flush := func() {
			if len(buf) == 0 {
				return
			}
			if err := s.store.AuditLogs().BatchCreate(context.Background(), buf); err != nil {
				s.logger.Error("audit flush failed", err)
			}
			buf = buf[:0]
		}

		for {
			select {
			case <-ctx.Done():
				// Drain remaining entries before exit.
				for {
					select {
					case e := <-s.ch:
						cp := e
						buf = append(buf, &cp)
					default:
						flush()
						return
					}
				}
			case e := <-s.ch:
				cp := e
				buf = append(buf, &cp)
				if len(buf) >= 100 {
					flush()
				}
			case <-ticker.C:
				flush()
			}
		}
	}()
}
