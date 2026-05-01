package monitor

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/monitor/adapter"
	"github.com/alatticeio/lattice/internal/monitor/alert"
	"github.com/alatticeio/lattice/internal/monitor/alert/notifier"
	"github.com/alatticeio/lattice/internal/monitor/gateway"
	"github.com/alatticeio/lattice/internal/monitor/template"
	"gorm.io/gorm"
)

// Monitor is the top-level monitoring component that wires together
// the query gateway, template registry, and alert engine.
type Monitor struct {
	Gateway     *gateway.MonitorGateway
	AlertEngine *alert.AlertEngine
	Templates   *template.TemplateRegistry
}

// NewMonitor creates a Monitor with VM and heartbeat adapters.
// If vmAddress is empty or heartbeatDB is nil, the corresponding adapter is skipped.
func NewMonitor(vmAddress string, st store.Store, heartbeatDB *gorm.DB) (*Monitor, error) {
	logger := log.GetLogger("monitor")

	templates, err := template.NewRegistry()
	if err != nil {
		return nil, err
	}

	gw := gateway.NewMonitorGateway()

	if vmAddress != "" {
		vmAdapter, err := adapter.NewVMAdapter(vmAddress, templates)
		if err != nil {
			logger.Warn("VM adapter init failed, VM queries will be unavailable", "err", err)
		} else {
			gw.Register(vmAdapter)
			logger.Info("VM adapter registered", "address", vmAddress)
		}
	}

	if heartbeatDB != nil {
		gw.Register(adapter.NewHeartbeatAdapter(heartbeatDB))
		logger.Info("heartbeat adapter registered")
	}

	notifiers := map[string]notifier.Notifier{
		"webhook":  notifier.NewWebhookNotifier(),
		"dingtalk": notifier.NewDingTalkNotifier(),
		"slack":    notifier.NewSlackNotifier(),
		"email":    notifier.NewEmailNotifier(),
	}

	alertEngine := alert.NewEngine(gw, st, notifiers)

	return &Monitor{
		Gateway:     gw,
		AlertEngine: alertEngine,
		Templates:   templates,
	}, nil
}

// StartAlertEngine launches the alert evaluation loop in a background goroutine.
// It stops when ctx is cancelled.
func (m *Monitor) StartAlertEngine(ctx context.Context) {
	go m.AlertEngine.Start(ctx)
}
