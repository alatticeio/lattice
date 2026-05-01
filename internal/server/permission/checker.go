// Copyright 2024 alatticeio
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package permission

import (
	"context"
	"errors"
	"fmt"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Checker enforces workspace-level role-based access and provides
// K8s impersonated clients scoped to the caller's permissions.
type Checker interface {
	// RequireWorkspaceRole checks that the caller has at least `role` in wsID.
	// Platform admins bypass membership check entirely.
	// Returns the member record for downstream use (nil for platform admins).
	RequireWorkspaceRole(ctx context.Context, wsID, userID string, role dto.WorkspaceRole) (*models.WorkspaceMember, error)

	// K8sClient returns an impersonated K8s client scoped to the caller's
	// workspace role. Requires membership and sufficient role.
	K8sClient(ctx context.Context, wsID, userID string) (client.Client, error)
}

type checker struct {
	store        store.Store
	impersonator *resource.IdentityImpersonator
}

// NewChecker creates a new permission Checker.
func NewChecker(st store.Store, impersonator *resource.IdentityImpersonator) Checker {
	return &checker{store: st, impersonator: impersonator}
}

// NewForTest creates a checker for testing (nil impersonator skips K8sClient).
func NewForTest(st store.Store, impersonator *resource.IdentityImpersonator) Checker {
	return &checker{store: st, impersonator: impersonator}
}

func (c *checker) RequireWorkspaceRole(ctx context.Context, wsID, userID string, role dto.WorkspaceRole) (*models.WorkspaceMember, error) {
	// Fetch user to check for platform admin.
	user, err := c.store.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Platform admins bypass workspace role checks.
	if user.SystemRole == dto.SystemRolePlatformAdmin {
		return nil, nil
	}

	// Check membership and status.
	member, err := c.store.WorkspaceMembers().GetMembership(ctx, wsID, userID)
	if err != nil {
		return nil, errors.New("not a member of this workspace")
	}

	// Reject suspended/removed members.
	if member.Status == models.MemberStatusSuspended || member.Status == models.MemberStatusRemoved {
		return nil, errors.New("your access to this workspace has been suspended or revoked")
	}

	if dto.GetRoleWeight(member.Role) < dto.GetRoleWeight(role) {
		return nil, errors.New("insufficient role in workspace")
	}

	return member, nil
}

func (c *checker) K8sClient(ctx context.Context, wsID, userID string) (client.Client, error) {
	member, err := c.RequireWorkspaceRole(ctx, wsID, userID, dto.RoleViewer)
	if err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	role := string(member.Role)
	return c.impersonator.NamespaceAccessor(wsID, userID, role)
}
