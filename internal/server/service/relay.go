package service

import (
	"context"
	"fmt"
	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/resource"
	"github.com/alatticeio/lattice/internal/server/vo"
	"net"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RelayService manages LatticeRelayServer CRDs.
type RelayService interface {
	List(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.RelayVo], error)
	Create(ctx context.Context, req *dto.RelayDto) (*vo.RelayVo, error)
	Update(ctx context.Context, id string, req *dto.RelayDto) (*vo.RelayVo, error)
	Delete(ctx context.Context, id string) error
	Test(ctx context.Context, id string) (*vo.RelayTestVo, error)
}

type relayService struct {
	log    *log.Logger
	client *resource.Client
	store  store.Store
}

// NewRelayService constructs a RelayService.
func NewRelayService(c *resource.Client, st store.Store) RelayService {
	return &relayService{
		log:    log.GetLogger("relay-service"),
		client: c,
		store:  st,
	}
}

// --------------------------------------------------------------------------
// List
// --------------------------------------------------------------------------

func (s *relayService) List(ctx context.Context, pageParam *dto.PageRequest) (*dto.PageResult[vo.RelayVo], error) {
	var list v1alpha1.LatticeRelayServerList
	if err := s.client.GetAPIReader().List(ctx, &list); err != nil {
		return nil, fmt.Errorf("relay list: %w", err)
	}

	all := make([]*vo.RelayVo, 0, len(list.Items))
	for i := range list.Items {
		all = append(all, relayToVo(&list.Items[i]))
	}

	// keyword filter
	if kw := strings.TrimSpace(pageParam.Keyword); kw != "" {
		kw = strings.ToLower(kw)
		filtered := all[:0]
		for _, r := range all {
			if strings.Contains(strings.ToLower(r.Name), kw) ||
				strings.Contains(strings.ToLower(r.TcpUrl), kw) ||
				strings.Contains(strings.ToLower(r.QuicUrl), kw) {
				filtered = append(filtered, r)
			}
		}
		all = filtered
	}

	total := len(all)
	page, size := pageParam.Page, pageParam.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	start := (page - 1) * size
	end := start + size
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	items := make([]vo.RelayVo, 0, end-start)
	for _, r := range all[start:end] {
		items = append(items, *r)
	}
	return &dto.PageResult[vo.RelayVo]{
		Page:     page,
		PageSize: size,
		Total:    int64(total),
		List:     items,
	}, nil
}

// --------------------------------------------------------------------------
// Create
// --------------------------------------------------------------------------

func (s *relayService) Create(ctx context.Context, req *dto.RelayDto) (*vo.RelayVo, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("relay name is required")
	}
	if req.TcpUrl == "" {
		return nil, fmt.Errorf("tcpUrl is required")
	}

	ns, err := s.resolveNamespaces(ctx, req.Workspaces)
	if err != nil {
		return nil, err
	}

	username, _ := ctx.Value(infra.UsernameKey).(string)
	now := time.Now().UTC().Format(time.RFC3339)

	obj := &v1alpha1.LatticeRelayServer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "alattice.io/v1alpha1",
			Kind:       "LatticeRelayServer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: req.Name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "lattice-controller",
			},
			Annotations: map[string]string{
				"alattice.io/created-by": username,
				"alattice.io/updated-by": username,
				"alattice.io/updated-at": now,
			},
		},
		Spec: v1alpha1.LatticeRelayServerSpec{
			DisplayName: req.DisplayName,
			Description: req.Description,
			TcpUrl:      req.TcpUrl,
			QuicUrl:     req.QuicUrl,
			Enabled:     req.Enabled,
			Namespaces:  ns,
		},
	}

	if err = s.client.Create(ctx, obj); err != nil {
		return nil, fmt.Errorf("relay create: %w", err)
	}
	return relayToVo(obj), nil
}

// --------------------------------------------------------------------------
// Update
// --------------------------------------------------------------------------

func (s *relayService) Update(ctx context.Context, id string, req *dto.RelayDto) (*vo.RelayVo, error) {
	var existing v1alpha1.LatticeRelayServer
	if err := s.client.Get(ctx, client.ObjectKey{Name: id}, &existing); err != nil {
		return nil, fmt.Errorf("relay get: %w", err)
	}

	ns, err := s.resolveNamespaces(ctx, req.Workspaces)
	if err != nil {
		return nil, err
	}

	username, _ := ctx.Value(infra.UsernameKey).(string)

	patch := existing.DeepCopy()
	patch.Spec.DisplayName = req.DisplayName
	patch.Spec.Description = req.Description
	patch.Spec.TcpUrl = req.TcpUrl
	patch.Spec.QuicUrl = req.QuicUrl
	patch.Spec.Enabled = req.Enabled
	patch.Spec.Namespaces = ns
	if patch.Annotations == nil {
		patch.Annotations = make(map[string]string)
	}
	patch.Annotations["alattice.io/updated-by"] = username
	patch.Annotations["alattice.io/updated-at"] = time.Now().UTC().Format(time.RFC3339)

	if err = s.client.Patch(ctx, patch, client.MergeFrom(&existing),
		client.FieldOwner("lattice-management")); err != nil {
		return nil, fmt.Errorf("relay update: %w", err)
	}
	return relayToVo(patch), nil
}

// --------------------------------------------------------------------------
// Delete
// --------------------------------------------------------------------------

func (s *relayService) Delete(ctx context.Context, id string) error {
	obj := &v1alpha1.LatticeRelayServer{
		ObjectMeta: metav1.ObjectMeta{Name: id},
	}
	if err := client.IgnoreNotFound(s.client.Delete(ctx, obj)); err != nil {
		return fmt.Errorf("relay delete: %w", err)
	}
	return nil
}

// --------------------------------------------------------------------------
// Test
// --------------------------------------------------------------------------

// Test performs a TCP dial to the relay's TcpUrl and reports latency.
func (s *relayService) Test(ctx context.Context, id string) (*vo.RelayTestVo, error) {
	var relay v1alpha1.LatticeRelayServer
	if err := s.client.Get(ctx, client.ObjectKey{Name: id}, &relay); err != nil {
		return nil, fmt.Errorf("relay get: %w", err)
	}

	addr := relay.Spec.TcpUrl
	if addr == "" {
		return &vo.RelayTestVo{OK: false, Error: "tcpUrl is empty"}, nil
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &vo.RelayTestVo{OK: false, LatencyMs: latency, Error: err.Error()}, nil
	}
	_ = conn.Close()
	return &vo.RelayTestVo{OK: true, LatencyMs: latency}, nil
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// resolveNamespaces translates workspace IDs to K8s namespace names.
// If the ID matches a namespace directly (e.g. already a namespace slug), it
// is used as-is.  Unknown IDs are silently skipped so a stale binding doesn't
// block the operation.
func (s *relayService) resolveNamespaces(ctx context.Context, workspaceIDs []string) ([]string, error) {
	if len(workspaceIDs) == 0 {
		return nil, nil
	}
	result := make([]string, 0, len(workspaceIDs))
	for _, id := range workspaceIDs {
		ws, err := s.store.Workspaces().GetByID(ctx, id)
		if err != nil {
			// ID might already be a namespace name; use it directly
			result = append(result, id)
			continue
		}
		if ws.Namespace != "" {
			result = append(result, ws.Namespace)
		}
	}
	return result, nil
}

func relayToVo(r *v1alpha1.LatticeRelayServer) *vo.RelayVo {
	v := &vo.RelayVo{
		ID:             r.Name,
		Name:           r.Spec.DisplayName,
		Description:    r.Spec.Description,
		TcpUrl:         r.Spec.TcpUrl,
		QuicUrl:        r.Spec.QuicUrl,
		Enabled:        r.Spec.Enabled,
		ConnectedPeers: r.Status.ConnectedPeers,
		LatencyMs:      r.Status.LatencyMs,
		Workspaces:     r.Spec.Namespaces,
		CreatedAt:      r.CreationTimestamp.Time,
	}
	ann := r.Annotations
	if ann == nil {
		ann = map[string]string{}
	}
	v.CreatedBy = ann["alattice.io/created-by"]
	v.UpdatedBy = ann["alattice.io/updated-by"]
	if ts := ann["alattice.io/updated-at"]; ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			v.UpdatedAt = t
		}
	}
	// normalise health to lower-case for frontend consistency
	switch r.Status.Health {
	case v1alpha1.RelayHealthHealthy:
		v.Status = "healthy"
	case v1alpha1.RelayHealthDegraded:
		v.Status = "degraded"
	case v1alpha1.RelayHealthOffline:
		v.Status = "offline"
	default:
		v.Status = "unknown"
	}
	return v
}
