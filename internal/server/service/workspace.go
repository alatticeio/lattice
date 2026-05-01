package service

import (
	"context"
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	client_r "github.com/alatticeio/lattice/internal/server/resource"
	"github.com/alatticeio/lattice/internal/server/vo"
	"github.com/alatticeio/lattice/pkg/utils"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/alatticeio/lattice/api/v1alpha1"
)

type WorkspaceService interface {
	AddWorkspace(ctx context.Context, dto *dto.WorkspaceDto) (*vo.WorkspaceVo, error)
	UpdateWorkspace(ctx context.Context, id string, dto *dto.WorkspaceDto) (*vo.WorkspaceVo, error)
	DeleteWorkspace(ctx context.Context, id string) error
	ListWorkspaces(ctx context.Context, search *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error)
}

type WorkspaceMemberService interface {
	Create(ctx context.Context, workspace *models.WorkspaceMember) (*models.WorkspaceMember, error)
	Update(ctx context.Context, workspace *models.WorkspaceMember) (*models.WorkspaceMember, error)
	Delete(ctx context.Context, workspace *models.WorkspaceMember) error
	List(ctx context.Context, workspaceID string) ([]*models.WorkspaceMember, error)

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
	// 先查出 namespace，再删 K8s 资源，最后删 DB 记录
	ws, err := w.store.Workspaces().GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 删除 K8s Namespace（级联删除其中所有资源：Peer、Network、Policy 等）
	if ws.Namespace != "" {
		ns := &corev1.Namespace{}
		ns.Name = ws.Namespace
		if err := w.client.Delete(ctx, ns); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete namespace %s: %w", ws.Namespace, err)
		}
	}

	return w.store.Tx(ctx, func(s store.Store) error {
		if err := s.WorkspaceMembers().DeleteByWorkspace(ctx, id); err != nil {
			return err
		}
		return s.Workspaces().Delete(ctx, id)
	})
}

func (w *workspaceService) ListWorkspaces(ctx context.Context, request *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error) {
	systemRole, _ := ctx.Value(infra.SystemRoleKey).(string)

	var (
		workspaces []*models.Workspace
		total      int64
		err        error
	)

	if systemRole == "platform_admin" {
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
				Slug:        ws.Slug,
				DisplayName: ws.DisplayName,
				Namespace:   ws.Namespace,
				Status:      "active",
				CreatedAt:   ws.CreatedAt.Format("2006-01-02T15:04:05Z"),
				CreatedBy:   ws.CreatedBy,
				UpdatedBy:   ws.UpdatedBy,
				UpdatedAt:   ws.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			}

			// 首先检查Namespace是否存在
			ns := &corev1.Namespace{}
			nsKey := client.ObjectKey{Name: ws.Namespace}

			if err := w.client.GetAPIReader().Get(gCtx, nsKey, ns); err != nil {
				// Namespace不存在，workspace未初始化
				v.Status = "inactive"
				v.NodeCount = 0
				v.QuotaUsage = 0
			} else {
				// Namespace存在，尝试获取ResourceQuota
				quota := &corev1.ResourceQuota{}
				quotaKey := client.ObjectKey{Name: "workspace-quota", Namespace: ws.Namespace}

				// 使用 GetAPIReader 确保获取最新数据
				if err := w.client.GetAPIReader().Get(gCtx, quotaKey, quota); err == nil {
					nodeRes := corev1.ResourceName("count/nodes.alattice.io")
					if hard, ok := quota.Status.Hard[nodeRes]; ok {
						v.NodeCount = hard.Value()
					}
					if used, ok := quota.Status.Used[nodeRes]; ok {
						v.QuotaUsage = used.Value()
					}
				} else {
					// ResourceQuota可能不存在或未就绪，但Namespace存在，所以workspace是active的
					v.NodeCount = 0
					v.QuotaUsage = 0
				}

				// 查询默认网络信息 - 使用 GetAPIReader 确保获取最新数据
				network := &v1alpha1.LatticeNetwork{}
				networkKey := client.ObjectKey{Name: "lattice-default-net", Namespace: ws.Namespace}
				if err := w.client.GetAPIReader().Get(gCtx, networkKey, network); err == nil {
					v.NetworkName = network.Spec.Name
					// 优先使用 Status.ActiveCIDR（Controller 实际分配的），如果没有则使用 Spec.CIDR
					if network.Status.ActiveCIDR != "" {
						v.NetworkCIDR = network.Status.ActiveCIDR
					} else {
						v.NetworkCIDR = network.Spec.CIDR
					}
					v.NetworkStatus = string(network.Status.Phase)
				}

				// 统计 EnrollmentToken 数量
				var tokenList v1alpha1.LatticeEnrollmentTokenList
				if err := w.client.GetAPIReader().List(gCtx, &tokenList, client.InNamespace(ws.Namespace)); err == nil {
					v.TokenCount = int64(len(tokenList.Items))
				}
			}

			result[idx] = v
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("k8s data aggregation failed: %v", err)
	}

	// 按状态过滤（status 由 k8s 动态计算，只能在丰富化后过滤）
	if request.Status != "" {
		filtered := result[:0]
		for _, v := range result {
			if v.Status == request.Status {
				filtered = append(filtered, v)
			}
		}
		result = filtered
		total = int64(len(result))
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

func (w *workspaceMemberService) Create(ctx context.Context, member *models.WorkspaceMember) (*models.WorkspaceMember, error) {
	if err := w.store.WorkspaceMembers().AddMember(ctx, member); err != nil {
		return nil, err
	}
	return member, nil
}

func (w *workspaceMemberService) Update(ctx context.Context, member *models.WorkspaceMember) (*models.WorkspaceMember, error) {
	if err := w.store.WorkspaceMembers().UpdateRole(ctx, member.WorkspaceID, member.UserID, member.Role); err != nil {
		return nil, err
	}
	return member, nil
}

func (w *workspaceMemberService) Delete(ctx context.Context, member *models.WorkspaceMember) error {
	return w.store.WorkspaceMembers().SoftRemove(ctx, member.WorkspaceID, member.UserID)
}

func (w *workspaceMemberService) List(ctx context.Context, workspaceID string) ([]*models.WorkspaceMember, error) {
	return w.store.WorkspaceMembers().ListMembers(ctx, workspaceID)
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

func (w *workspaceService) AddWorkspace(ctx context.Context, dto *dto.WorkspaceDto) (*vo.WorkspaceVo, error) {
	userID, _ := ctx.Value(infra.UserIDKey).(string)
	slug := utils.GenerateSlug(dto.Slug)

	// 同一用户下不允许重名（跨用户允许）
	if userID != "" {
		exists, err := w.store.Workspaces().ExistsByUserAndSlug(ctx, userID, slug)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("workspace with name %q already exists", dto.Slug)
		}
	}

	username, _ := ctx.Value(infra.UsernameKey).(string)

	var res vo.WorkspaceVo
	err := w.store.Tx(ctx, func(s store.Store) error {
		newWs := &models.Workspace{
			Slug:        slug,
			DisplayName: dto.DisplayName,
			CreatedBy:   username,
			// 先不设置 Namespace，等创建后再更新
		}
		if err := s.Workspaces().Create(ctx, newWs); err != nil {
			return err
		}

		// 生成实际的 Namespace 名称
		nsName := fmt.Sprintf("wf-%s", newWs.ID)
		newWs.Namespace = nsName

		// 更新数据库中的 Namespace 字段
		if err := s.Workspaces().Update(ctx, newWs); err != nil {
			return err
		}

		// 初始化 K8s 资源
		if err := w.InitNewNamespace(ctx, newWs.ID); err != nil {
			return err
		}

		// 将创建者加为 admin 成员，使后续重名校验（join t_workspaces_member）能正确生效
		if userID != "" {
			now := time.Now()
			if err := s.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
				WorkspaceID: newWs.ID,
				UserID:      userID,
				Role:        "admin",
				Status:      "active",
				JoinedAt:    &now,
			}); err != nil {
				return err
			}
		}

		res = vo.WorkspaceVo{ID: newWs.ID, Slug: newWs.Slug, Namespace: newWs.Namespace, DisplayName: newWs.DisplayName, Status: "active"}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (w *workspaceService) UpdateWorkspace(ctx context.Context, id string, dto *dto.WorkspaceDto) (*vo.WorkspaceVo, error) {
	username, _ := ctx.Value(infra.UsernameKey).(string)

	ws, err := w.store.Workspaces().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if dto.DisplayName != "" {
		ws.DisplayName = dto.DisplayName
	}
	ws.UpdatedBy = username

	if err := w.store.Workspaces().Update(ctx, ws); err != nil {
		return nil, err
	}

	return &vo.WorkspaceVo{
		ID:          ws.ID,
		Slug:        ws.Slug,
		Namespace:   ws.Namespace,
		DisplayName: ws.DisplayName,
		Status:      ws.Status,
		CreatedBy:   ws.CreatedBy,
		UpdatedBy:   ws.UpdatedBy,
		UpdatedAt:   ws.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (w *workspaceService) InitNewNamespace(ctx context.Context, workspaceId string) error {
	return w.InitializeTenant(ctx, workspaceId, "admin")
}

func (w *workspaceService) InitializeTenant(ctx context.Context, wsID, role string) error {
	nsName := fmt.Sprintf("wf-%s", wsID)

	// 1. 创建Namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nsName,
			Labels: map[string]string{"alattice.io/workspace-id": wsID},
		},
	}
	if err := w.client.Create(ctx, ns); err != nil {
		return err
	}

	// 2. 创建ResourceQuota
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "workspace-quota", Namespace: nsName},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceName("count/nodes.alattice.io"): resource.MustParse("50"),
				corev1.ResourceSecrets:                         resource.MustParse("20"),
			},
		},
	}
	if err := w.client.Create(ctx, quota); err != nil {
		return fmt.Errorf("failed to create quota: %v", err)
	}

	// 3. 创建RoleBinding
	for _, r := range []string{"admin", "editor", "member", "viewer"} {
		if err := w.createRoleBinding(ctx, nsName, wsID, r); err != nil {
			return fmt.Errorf("failed to create role binding: %v", err)
		}
	}

	// 4. 创建默认网络
	if err := w.createDefaultNetwork(ctx, nsName); err != nil {
		return fmt.Errorf("failed to create default network: %v", err)
	}

	// 5. 创建默认策略 (deny-all)
	if err := w.createDefaultPolicy(ctx, nsName); err != nil {
		return fmt.Errorf("failed to create default policy: %v", err)
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
			Name:     fmt.Sprintf("lattice-%s", roleName),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return w.client.Create(ctx, rb)
}

func (w *workspaceService) createDefaultNetwork(ctx context.Context, nsName string) error {
	var defaultNet v1alpha1.LatticeNetwork
	if err := w.client.Get(ctx, client.ObjectKey{Namespace: nsName, Name: "lattice-default-net"}, &defaultNet); err != nil {
		if k8serrors.IsNotFound(err) {
			defaultNet = v1alpha1.LatticeNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lattice-default-net",
					Namespace: nsName,
					Labels:    map[string]string{"app.kubernetes.io/managed-by": "lattice-controller"},
				},
				Spec: v1alpha1.LatticeNetworkSpec{
					Name: "lattice-default-net", // 使用固定的默认名称
					CIDR: "100.64.0.0/16",       // 设置默认 CIDR，使用 CGNAT 地址段
				},
			}

			if k8serr := w.client.Create(ctx, &defaultNet); k8serr != nil {
				return fmt.Errorf("failed to create default network: %v", k8serr)
			}
		} else {
			return err
		}
	}
	return nil
}

func (w *workspaceService) createDefaultPolicy(ctx context.Context, nsName string) error {
	var defaultPolicy v1alpha1.LatticePolicy
	if err := w.client.Get(ctx, client.ObjectKey{Namespace: nsName, Name: "lattice-deny-all"}, &defaultPolicy); err != nil {
		if k8serrors.IsNotFound(err) {
			defaultPolicy = v1alpha1.LatticePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lattice-deny-all",
					Namespace: nsName,
					Labels:    map[string]string{"app.kubernetes.io/managed-by": "lattice-controller"},
				},
				Spec: v1alpha1.LatticePolicySpec{
					Network: fmt.Sprintf("%s-net", nsName),
					Action:  "DENY",
				},
			}

			if k8serr := w.client.Create(ctx, &defaultPolicy); k8serr != nil {
				return fmt.Errorf("failed to create default policy: %v", k8serr)
			}
		} else {
			return err
		}
	}
	return nil
}
