package resource

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// internal/platform/k8s/impersonator.go

type IdentityImpersonator struct {
	baseConfig *rest.Config // Base high-privilege config
	scheme     *runtime.Scheme
}

func NewIdentityImpersonator() (*IdentityImpersonator, error) {
	// Similar to ctrl.GetConfigOrDie(), but here we handle errors
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}

	return &IdentityImpersonator{
		baseConfig: cfg,
		scheme:     scheme,
	}, nil
}

// NamespaceAccessor generates a restricted K8s client for a specific workspace and user
// wsID: workspace ID
// userID: caller's user ID (used for audit and RBAC group binding)
// role: business role (admin, editor, member, viewer)
func (i *IdentityImpersonator) NamespaceAccessor(wsID, userID, role string) (client.Client, error) {
	// 1. Deep copy the original rest.Config to avoid modifying the global config
	config := rest.CopyConfig(i.baseConfig)

	// 2. Construct identity impersonation
	// Username: used for audit logs, wf-user-<userID>
	// Group: this is key for RBAC validation, wf-group-<wsID>-<role>
	config.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("wf-user-%s", userID),
		Groups:   []string{fmt.Sprintf("wf-group-%s-%s", wsID, role)},
	}

	// 3. Create the client
	// Note: we use client.New here instead of i.client
	// because i.client typically has Cache, and impersonated access must be a Direct Client (requests API Server directly)
	scopedClient, err := client.New(config, client.Options{
		Scheme: i.scheme, // Scheme is required to parse native resources or CRDs
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create scoped k8s client: %w", err)
	}

	return scopedClient, nil
}
