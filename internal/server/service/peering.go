package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/resource"
	"github.com/alatticeio/lattice/internal/server/vo"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PeeringService interface {
	List(ctx context.Context) ([]vo.PeeringVo, error)
	Create(ctx context.Context, d *dto.PeeringDto) (*vo.PeeringVo, error)
	Delete(ctx context.Context, name string) error
}

type peeringService struct {
	client *resource.Client
	store  store.Store
}

func NewPeeringService(c *resource.Client, st store.Store) PeeringService {
	return &peeringService{client: c, store: st}
}

// List returns all LatticeNetworkPeerings that involve the current workspace's namespace.
func (s *peeringService) List(ctx context.Context) ([]vo.PeeringVo, error) {
	wsID, _ := ctx.Value(infra.WorkspaceKey).(string)
	ws, err := s.store.Workspaces().GetByID(ctx, wsID)
	if err != nil {
		return nil, err
	}
	currentNs := ws.Namespace

	var list v1alpha1.LatticeNetworkPeeringList
	if err := s.client.GetAPIReader().List(ctx, &list); err != nil {
		return nil, err
	}

	var vos []vo.PeeringVo
	for _, p := range list.Items {
		if p.Spec.NamespaceA != currentNs && p.Spec.NamespaceB != currentNs {
			continue
		}

		// Determine local vs remote sides from the current workspace's perspective.
		localNs := p.Spec.NamespaceA
		localNet := p.Spec.NetworkA
		remoteNs := p.Spec.NamespaceB
		remoteNet := p.Spec.NetworkB
		if p.Spec.NamespaceB == currentNs {
			localNs, localNet = p.Spec.NamespaceB, p.Spec.NetworkB
			remoteNs, remoteNet = p.Spec.NamespaceA, p.Spec.NetworkA
		}

		localEndpoint := s.buildEndpoint(ctx, localNs, localNet)
		remoteEndpoint := s.buildEndpoint(ctx, remoteNs, remoteNet)

		// Enrich with workspace display names.
		if localWs, err := s.store.Workspaces().GetByNamespace(ctx, localNs); err == nil {
			localEndpoint.Name = localWs.DisplayName
		}
		if remoteWs, err := s.store.Workspaces().GetByNamespace(ctx, remoteNs); err == nil {
			remoteEndpoint.Name = remoteWs.DisplayName
		}

		vos = append(vos, vo.PeeringVo{
			Name:        p.Name,
			Local:       localEndpoint,
			Remote:      remoteEndpoint,
			Status:      phaseToStatus(string(p.Status.Phase)),
			PeeringMode: string(p.Spec.PeeringMode),
			CreatedAt:   p.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return vos, nil
}

// Create creates a new LatticeNetworkPeering between the current workspace and the remote.
func (s *peeringService) Create(ctx context.Context, d *dto.PeeringDto) (*vo.PeeringVo, error) {
	wsID, _ := ctx.Value(infra.WorkspaceKey).(string)
	ws, err := s.store.Workspaces().GetByID(ctx, wsID)
	if err != nil {
		return nil, err
	}
	nsA := ws.Namespace
	nsB := d.NamespaceB

	// Resolve local network name.
	netA, err := s.defaultNetwork(ctx, nsA)
	if err != nil {
		return nil, fmt.Errorf("local network: %w", err)
	}

	// Remote network defaults to lattice-default-net.
	netB := d.NetworkB
	if netB == "" {
		netB = "lattice-default-net"
	}

	// Auto-generate name if not provided.
	name := d.Name
	if name == "" {
		name = fmt.Sprintf("%s-to-%s", nsA, nsB)
		// K8s names must be DNS-compliant.
		name = strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	}

	mode := v1alpha1.PeeringMode(d.PeeringMode)
	if mode == "" {
		mode = v1alpha1.PeeringModeGateway
	}

	peering := &v1alpha1.LatticeNetworkPeering{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.LatticeNetworkPeeringSpec{
			NamespaceA:  nsA,
			NetworkA:    netA,
			NamespaceB:  nsB,
			NetworkB:    netB,
			PeeringMode: mode,
		},
	}

	if err := s.client.Create(ctx, peering); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("peering %q already exists", name)
		}
		return nil, err
	}

	result := &vo.PeeringVo{
		Name:        peering.Name,
		Local:       s.buildEndpoint(ctx, nsA, netA),
		Remote:      s.buildEndpoint(ctx, nsB, netB),
		Status:      "pending",
		PeeringMode: string(mode),
		CreatedAt:   peering.CreationTimestamp.UTC().Format(time.RFC3339),
	}
	if result.CreatedAt == "0001-01-01T00:00:00Z" {
		result.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if ws.DisplayName != "" {
		result.Local.Name = ws.DisplayName
	}
	return result, nil
}

// Delete removes a LatticeNetworkPeering by name.
func (s *peeringService) Delete(ctx context.Context, name string) error {
	var peering v1alpha1.LatticeNetworkPeering
	if err := s.client.GetAPIReader().Get(ctx, client.ObjectKey{Name: name}, &peering); err != nil {
		return err
	}
	return s.client.Delete(ctx, &peering)
}

// defaultNetwork returns the first LatticeNetwork found in the namespace,
// preferring "lattice-default-net".
func (s *peeringService) defaultNetwork(ctx context.Context, ns string) (string, error) {
	var list v1alpha1.LatticeNetworkList
	if err := s.client.GetAPIReader().List(ctx, &list, client.InNamespace(ns)); err != nil {
		return "", err
	}
	for _, n := range list.Items {
		if n.Name == "lattice-default-net" {
			return n.Name, nil
		}
	}
	if len(list.Items) > 0 {
		return list.Items[0].Name, nil
	}
	return "lattice-default-net", nil
}

// buildEndpoint enriches a WorkspaceEndpointVo with CIDR and peer count from K8s.
func (s *peeringService) buildEndpoint(ctx context.Context, ns, networkName string) vo.WorkspaceEndpointVo {
	ep := vo.WorkspaceEndpointVo{
		Name:      ns,
		Namespace: ns,
	}

	var network v1alpha1.LatticeNetwork
	if err := s.client.GetAPIReader().Get(ctx, client.ObjectKey{Namespace: ns, Name: networkName}, &network); err == nil {
		ep.CIDR = network.Status.ActiveCIDR
	}

	var peerList v1alpha1.LatticePeerList
	if err := s.client.GetAPIReader().List(ctx, &peerList, client.InNamespace(ns)); err == nil {
		for _, p := range peerList.Items {
			if p.GetLabels()["alattice.io/shadow"] != "true" {
				ep.NodeCount++
			}
		}
	}
	return ep
}

// phaseToStatus maps LatticeNetworkPhase to the frontend status string.
func phaseToStatus(phase string) string {
	switch phase {
	case string(v1alpha1.NetworkPhaseReady):
		return "active"
	case string(v1alpha1.NetworkPhaseFailed):
		return "failed"
	default:
		return "pending"
	}
}
