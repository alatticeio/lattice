package service

import (
	"context"
	"errors"
	"fmt"
	"wireflow/internal/config"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	client_r "wireflow/management/resource"
	"wireflow/management/vo"
	"wireflow/pkg/utils"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkspaceService interface {
	OnboardExternalUser(ctx context.Context, userId, extEmail string) (*models.User, error)
	AddWorkspace(ctx context.Context, dto *dto.WorkspaceDto) (*vo.WorkspaceVo, error)
	DeleteWorkspace(ctx context.Context, id string) error
	ListWorkspaces(ctx context.Context, search *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error)
}

type WorkspaceMemberService interface {
	Create(ctx context.Context, workspace *models.WorkspaceMember) (*models.WorkspaceMember, error)
	Update(ctx context.Context, workspace *models.WorkspaceMember) (*models.WorkspaceMember, error)
	Delete(ctx context.Context, workspace *models.WorkspaceMember) error
	List(ctx context.Context) ([]*models.WorkspaceMember, error)

	// GetMemberRole 获取用户在特定工作区中的角色
	GetMemberRole(ctx context.Context, workspaceNamespace string, userID string) (dto.WorkspaceRole, error)
}

type workspaceService struct {
	log      *log.Logger
	client   *client_r.Client
	store    store.Store
	identify *client_r.IdentityImpersonator
}

func (w *workspaceService) DeleteWorkspace(ctx context.Context, id string) error {
	return w.store.Tx(ctx, func(s store.Store) error {
		if err := s.WorkspaceMembers().DeleteByWorkspace(ctx, id); err != nil {
			return err
		}
		return s.Workspaces().Delete(ctx, id)
	})
}

func (w *workspaceService) ListWorkspaces(ctx context.Context, request *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error) {
	userRole := "super_admin"

	var (
		workspaces []*models.Workspace
		total      int64
		err        error
	)

	if userRole == "super_admin" {
		workspaces, total, err = w.store.Workspaces().List(ctx, request.Keyword, request.Page, request.PageSize)
		if err != nil {
			return nil, err
		}
	} else {
		userId := ctx.Value(infra.UserIDKey).(string)
		var members []*models.WorkspaceMember
		members, total, err = w.store.WorkspaceMembers().ListByUser(ctx, userId, request.Page, request.PageSize)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			workspaces = append(workspaces, &m.Workspace)
		}
	}

	result := make([]vo.WorkspaceVo, len(workspaces))

	g, gCtx := errgroup.WithContext(ctx)

	for i, workspace := range workspaces {
		idx, ws := i, workspace

		g.Go(func() error {
			v := vo.WorkspaceVo{
				ID:          ws.ID,
				DisplayName: ws.DisplayName,
				Namespace:   ws.Namespace,
				Status:      "healthy",
			}

			quota := &corev1.ResourceQuota{}
			quotaKey := client.ObjectKey{Name: "workspace-quota", Namespace: ws.Namespace}

			err := w.client.Get(gCtx, quotaKey, quota)
			if err == nil {
				nodeRes := corev1.ResourceName("count/nodes.wireflowcontroller.wireflow.run")
				if hard, ok := quota.Status.Hard[nodeRes]; ok {
					v.NodeCount = hard.Value()
				}
				if used, ok := quota.Status.Used[nodeRes]; ok {
					v.QuotaUsage = used.Value()
				}
			} else {
				v.Status = "initializing"
				v.NodeCount = 0
				v.QuotaUsage = 0
			}

			result[idx] = v
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("k8s data aggregation failed: %v", err)
	}

	return &dto.PageResult[vo.WorkspaceVo]{
		PageSize: request.PageSize,
		Page:     request.Page,
		List:     result,
		Total:    total,
	}, nil
}

type workspaceMemberService struct {
	log   *log.Logger
	store store.Store
}

func (w *workspaceMemberService) GetMemberRole(ctx context.Context, workspaceNamespace string, userID string) (dto.WorkspaceRole, error) {
	ws, err := w.store.Workspaces().GetByNamespace(ctx, workspaceNamespace)
	if err != nil {
		return "", err
	}
	member, err := w.store.WorkspaceMembers().GetMembership(ctx, ws.ID, userID)
	if err != nil {
		return "", err
	}
	return member.Role, nil
}

func (w *workspaceMemberService) Create(ctx context.Context, workspace *models.WorkspaceMember) (*models.WorkspaceMember, error) {
	//TODO implement me
	panic("implement me")
}

func (w *workspaceMemberService) Update(ctx context.Context, workspace *models.WorkspaceMember) (*models.WorkspaceMember, error) {
	//TODO implement me
	panic("implement me")
}

func (w *workspaceMemberService) Delete(ctx context.Context, workspace *models.WorkspaceMember) error {
	//TODO implement me
	panic("implement me")
}

func (w *workspaceMemberService) List(ctx context.Context) ([]*models.WorkspaceMember, error) {
	//TODO implement me
	panic("implement me")
}

func NewWorkspaceService(client *client_r.Client, st store.Store) WorkspaceService {
	logger := log.GetLogger("team-service")
	identify, err := client_r.NewIdentityImpersonator()
	if err != nil {
		logger.Error("init identity impersonator failed", err)
	}
	return &workspaceService{
		log:      logger,
		identify: identify,
		client:   client,
		store:    st,
	}
}

func NewWorkspaceMemberService(st store.Store) WorkspaceMemberService {
	return &workspaceMemberService{
		log:   log.GetLogger("workspace-member-service"),
		store: st,
	}
}

func (w *workspaceService) OnboardExternalUser(ctx context.Context, externalID, email string) (*models.User, error) {
	existing, err := w.store.Users().GetByExternalID(ctx, externalID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	role := dto.RoleViewer
	for _, adminEmail := range config.GlobalConfig.App.InitAdmins {
		if email == adminEmail.Username {
			role = dto.RoleAdmin
			break
		}
	}

	user := &models.User{ExternalID: externalID, Email: email, Role: role}
	if err := w.store.Users().Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (w *workspaceService) AddWorkspace(ctx context.Context, dto *dto.WorkspaceDto) (*vo.WorkspaceVo, error) {
	var res vo.WorkspaceVo
	err := w.store.Tx(ctx, func(s store.Store) error {
		newWs := &models.Workspace{
			Slug:        utils.GenerateSlug(dto.Slug),
			DisplayName: dto.DisplayName,
			Namespace:   dto.Namespace,
		}
		if err := s.Workspaces().Create(ctx, newWs); err != nil {
			return err
		}
		if err := w.InitNewNamespace(ctx, newWs.ID); err != nil {
			return err
		}
		res = vo.WorkspaceVo{ID: newWs.ID, Namespace: dto.Namespace, DisplayName: dto.DisplayName}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (w *workspaceService) InitNewNamespace(ctx context.Context, workspaceId string) error {
	return w.InitializeTenant(ctx, workspaceId, "admin")
}

func (w *workspaceService) CreateRoleBinding(ctx context.Context, perm *models.UserNamespacePermission) error {
	return nil
}

func (w *workspaceService) InitializeTenant(ctx context.Context, wsID, role string) error {
	nsName := fmt.Sprintf("wf-%s", wsID)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nsName,
			Labels: map[string]string{"wireflow.run/workspace-id": wsID},
		},
	}
	if err := w.client.Create(ctx, ns); err != nil {
		return err
	}

	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "workspace-quota", Namespace: nsName},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceName("count/nodes.wireflowcontroller.wireflow.run"): resource.MustParse("50"),
				corev1.ResourceSecrets: resource.MustParse("20"),
			},
		},
	}
	if err := w.client.Create(ctx, quota); err != nil {
		return fmt.Errorf("failed to create quota: %v", err)
	}

	for _, r := range []string{"admin", "member", "viewer"} {
		if err := w.createRoleBinding(ctx, nsName, wsID, r); err != nil {
			return fmt.Errorf("failed to create role binding: %v", err)
		}
	}
	return nil
}

// nolint:all
func (w *workspaceService) setupQuota(ctx context.Context, ns string, plan *models.Plan) {
	quota := &corev1.ResourceQuota{
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourcePods:   resource.MustParse(plan.PeerLimit),
				corev1.ResourceMemory: resource.MustParse(plan.MemoryLimit),
			},
		},
	}
	w.client.Create(ctx, quota)
}

func (w *workspaceService) createRoleBinding(ctx context.Context, ns, wsID, roleName string) error {
	rbName := fmt.Sprintf("wf-rb-%s-%s", wsID, roleName)
	groupName := fmt.Sprintf("wf-group-%s-%s", wsID, roleName)

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: ns},
		Subjects: []rbacv1.Subject{{
			Kind:     "Group",
			Name:     groupName,
			APIGroup: "rbac.authorization.k8s.io",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     fmt.Sprintf("wireflow-%s", roleName),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return w.client.Create(ctx, rb)
}
