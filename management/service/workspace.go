package service

import (
	"context"
	"errors"
	"fmt"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/management/database"
	"wireflow/management/dto"
	"wireflow/management/model"
	"wireflow/management/repository"
	client_r "wireflow/management/resource"
	"wireflow/management/vo"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkspaceService interface {
	OnboardExternalUser(ctx context.Context, userId, extEmail string) (*model.User, error)
	AddWorkspace(ctx context.Context, dto *dto.WorkspaceDto) (*vo.WorkspaceVo, error)
	ListWorkspaces(ctx context.Context, search *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error)
}

type WorkspaceMemberService interface {
	Create(ctx context.Context, workspace *model.WorkspaceMember) (*model.WorkspaceMember, error)
	Update(ctx context.Context, workspace *model.WorkspaceMember) (*model.WorkspaceMember, error)
	Delete(ctx context.Context, workspace *model.WorkspaceMember) error
	List(ctx context.Context) ([]*model.WorkspaceMember, error)

	// GetMemberRole 获取用户在特定工作区中的角色
	GetMemberRole(ctx context.Context, workspaceSlug string, userID string) (dto.WorkspaceRole, error)
}

type workspaceService struct {
	log           *log.Logger
	client        *client_r.Client
	db            *gorm.DB // SQLite 实例
	config        *config.Config
	workspaceRepo repository.WorkspaceRepository
	memberRepo    repository.WorkspaceMemberRepository
	identify      *client_r.IdentityImpersonator
}

func (w *workspaceService) ListWorkspaces(ctx context.Context, request *dto.PageRequest) (*dto.PageResult[vo.WorkspaceVo], error) {
	userRole := "super_admin"

	var workspaces []model.Workspace
	var total int64
	var err error

	if userRole == "super_admin" {
		workspaces, total, err = w.workspaceRepo.List(ctx, request)
		if err != nil {
			return nil, err
		}
	} else {
		var members []model.WorkspaceMember
		members, total, err = w.memberRepo.List(ctx, request)
		if err != nil {
			return nil, err
		}

		for _, m := range members {
			workspaces = append(workspaces, m.Workspace)
		}
	}

	result := make([]vo.WorkspaceVo, len(workspaces))

	g, gCtx := errgroup.WithContext(ctx)

	for i, workspace := range workspaces {
		// 闭包变量处理
		idx, ws := i, workspace

		g.Go(func() error {
			//nsName := fmt.Sprintf("wf-%s", ws.ID)

			// 初始化基础 VO 信息
			v := vo.WorkspaceVo{
				ID:          ws.ID,
				DisplayName: ws.DisplayName,
				Namespace:   ws.Namespace,
				Status:      "healthy",
			}

			// 使用系统高权限 Client 查询 Quota (因为 view 角色可能没权限查 ResourceQuota 对象)
			quota := &corev1.ResourceQuota{}
			quotaKey := client.ObjectKey{Name: "workspace-quota", Namespace: ws.Namespace}

			err := w.client.Get(gCtx, quotaKey, quota)
			if err == nil {
				// 提取我们在 InitializeTenant 中定义的自定义资源配额
				// 对应 count/nodes.wireflowcontroller.wireflow.run
				nodeRes := corev1.ResourceName("count/nodes.wireflowcontroller.wireflow.run")

				if hard, ok := quota.Status.Hard[nodeRes]; ok {
					v.NodeCount = hard.Value()
				}
				if used, ok := quota.Status.Used[nodeRes]; ok {
					v.QuotaUsage = used.Value()
				}
			} else {
				// 如果 Namespace 还没初始化完成或 Quota 被删除了
				v.Status = "initializing"
				v.NodeCount = 0
				v.QuotaUsage = 0
			}

			result[idx] = v
			return nil
		})
	}

	// 等待所有并发 K8s 请求结束
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
	log                 *log.Logger
	workspaceMemberRepo repository.WorkspaceMemberRepository
}

func (w *workspaceMemberService) GetMemberRole(ctx context.Context, workspaceSlug string, userID string) (dto.WorkspaceRole, error) {
	return w.workspaceMemberRepo.GetMemberRole(ctx, workspaceSlug, userID)
}

func (w *workspaceMemberService) Create(ctx context.Context, workspace *model.WorkspaceMember) (*model.WorkspaceMember, error) {
	//TODO implement me
	panic("implement me")
}

func (w *workspaceMemberService) Update(ctx context.Context, workspace *model.WorkspaceMember) (*model.WorkspaceMember, error) {
	//TODO implement me
	panic("implement me")
}

func (w *workspaceMemberService) Delete(ctx context.Context, workspace *model.WorkspaceMember) error {
	//TODO implement me
	panic("implement me")
}

func (w *workspaceMemberService) List(ctx context.Context) ([]*model.WorkspaceMember, error) {
	//TODO implement me
	panic("implement me")
}

func NewWorkspaceService(client *client_r.Client) WorkspaceService {
	logger := log.GetLogger("team-service")
	identify, err := client_r.NewIdentityImpersonator()

	if err != nil {
		logger.Error("init identity impersonator failed", err)
	}
	return &workspaceService{
		log:           logger,
		identify:      identify,
		client:        client,
		db:            database.DB,
		workspaceRepo: repository.NewWorkspaceRepository(),
		memberRepo:    repository.NewWorkspaceMemberRepository(),
	}
}

func NewWorkspaceMemberService() WorkspaceMemberService {
	return &workspaceMemberService{
		log:                 log.GetLogger("workspace-member-service"),
		workspaceMemberRepo: repository.NewWorkspaceMemberRepository(),
	}
}

func (w *workspaceService) OnboardExternalUser(ctx context.Context, externalID, email string) (*model.User, error) {
	var user model.User

	// 1. 先查数据库
	err := w.db.Where("external_id = ? OR email = ?", externalID, email).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 2. 如果是新用户，判断是否应该给予 Admin 角色
		role := dto.RoleViewer // 默认是普通用户
		for _, adminEmail := range config.GlobalConfig.App.InitAdmins {
			if email == adminEmail.Username {
				role = dto.RoleAdmin
				break
			}
		}

		// 3. 构造新用户对象
		user = model.User{
			ExternalID: externalID,
			Email:      email,
			Role:       role,
		}
		// 数据库
		if err := w.db.Create(&user).Error; err != nil {
			return nil, err
		}

	}

	return &user, nil
}

// CreateTeamWithInfrastructure 创建 K8s Namespace、Quota、RoleBinding
func (w *workspaceService) CreateTeamWithInfrastructure(ctx context.Context, ownerID string, teamName string) error {
	teamID := fmt.Sprintf("wf-team-%s", ownerID[:8]) // 确保 ID 兼容 DNS 命名

	return w.db.Transaction(func(tx *gorm.DB) error {
		// --- A. SQLite 事务 ---
		team := model.Workspace{DisplayName: teamName}

		if err := tx.Create(&team).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.WorkspaceMember{WorkspaceID: teamID, UserID: ownerID, Role: "admin"}).Error; err != nil {
			return err
		}

		// --- B. K8s 基础设施下发 ---

		// 1. 创建 Namespace
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   teamID,
			Labels: map[string]string{"managed-by": "wireflow"},
		}}
		if err := w.client.Create(ctx, ns); err != nil {
			return err
		}

		// 2. 创建 ResourceQuota (限制 CRD 数量)
		quota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: "team-quota", Namespace: teamID},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceName("count/nodes.wireflow.io"): resource.MustParse("50"),
					corev1.ResourceSecrets:                         resource.MustParse("20"),
				},
			},
		}
		if err := w.client.Create(ctx, quota); err != nil {
			return err
		}

		// 3. 创建 RoleBinding (将 Owner 绑定到 Namespace)
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "owner-binding", Namespace: teamID},
			Subjects: []rbacv1.Subject{{
				Kind:     "User",
				Name:     ownerID, // 这里对应外部身份系统的 ID/Name
				APIGroup: "rbac.authorization.k8s.io",
			}},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     "wf-resource-editor", // 引用之前定义的模板
				APIGroup: "rbac.authorization.k8s.io",
			},
		}
		return w.client.Create(ctx, rb)
	})
}

// teamService逻辑
func (w *workspaceService) AddWorkspace(ctx context.Context, dto *dto.WorkspaceDto) (*vo.WorkspaceVo, error) {

	var res vo.WorkspaceVo
	//开一个事务来操作
	err := w.db.Transaction(func(tx *gorm.DB) error {
		var err error
		txRepo := w.workspaceRepo.WithTx(tx)

		ws, err := txRepo.Create(ctx, dto)
		if err != nil {
			return err
		}

		// 1. 在 K8s 中真实创建 Namespace, 创建RoleBinding
		// 同时可以创建 ResourceQuota (配额), 限制用户创建的节点数与空间数，防止用户把集群搞崩
		err = w.InitNewNamespace(ctx, ws.ID)
		if err != nil {
			return err
		}

		res = vo.WorkspaceVo{
			Namespace:   dto.Namespace,
			DisplayName: dto.DisplayName,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &res, nil
}

// 授权逻辑
func (w *workspaceService) AssignPermission(ctx context.Context, dto *dto.UserNamespacePermissionDto) error {

	// 1. 数据库写入权限记录
	perm := &model.UserNamespacePermission{
		UserID:      dto.UserID,
		Namespace:   dto.Namespace,
		AccessLevel: dto.AccessLevel,
	}
	w.db.Save(&perm)

	// 2. 【高级操作】同步创建 K8s RBAC RoleBinding
	// 这样该用户以后通过 kubectl 访问时也会被权限限制
	err := w.CreateRoleBinding(ctx, perm)
	if err != nil {
		return err
	}

	return nil
}

func (w *workspaceService) InitNewNamespace(ctx context.Context, workspaceId string) error {
	return w.InitializeTenant(ctx, workspaceId, "admin")
}

func (w *workspaceService) CreateRoleBinding(ctx context.Context, perm *model.UserNamespacePermission) error {
	return nil
}

// 创建资源
func (w *workspaceService) InitializeTenant(ctx context.Context, wsID, role string) error {
	nsName := fmt.Sprintf("wf-%s", wsID)

	// 构造 Namespace 对象
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				"wireflow.run/workspace-id": wsID, // 建议加上标签，方便以后清理
			},
		},
	}

	// 1. 创建 Namespace， 这里不能用模拟的client
	err := w.client.Create(ctx, ns)
	if err != nil {
		return err
	}
	//创建quota
	// 2. 创建 ResourceQuota (限制 CRD 数量)
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "workspace-quota", Namespace: nsName},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceName("count/nodes.wireflowcontroller.wireflow.run"): resource.MustParse("50"),
				corev1.ResourceSecrets: resource.MustParse("20"),
			},
		},
	}

	err = w.client.Create(ctx, quota)
	if err != nil {
		return fmt.Errorf("failed to create quota: %v", err)
	}

	// 2. 为三个角色创建 RoleBinding
	// 假设你已经预先手动或在集群启动时创建了 3 个全局 ClusterRole:
	// wireflow-admin, wireflow-member, wireflow-viewer
	roles := []string{"admin", "member", "viewer"}
	for _, role := range roles {
		err = w.createRoleBinding(ctx, nsName, wsID, role)
		if err != nil {
			return fmt.Errorf("failed to create role binding: %v", err)
		}
	}
	return nil
}

// 无论什么用户，都走这套逻辑，只是参数不同
func (w *workspaceService) setupQuota(ctx context.Context, ns string, plan *model.Plan) {
	quota := &corev1.ResourceQuota{
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourcePods:   resource.MustParse(plan.PeerLimit),   // 免费版 5个，专业版 100个
				corev1.ResourceMemory: resource.MustParse(plan.MemoryLimit), // 免费版 1Gi，专业版 32Gi
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
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     groupName, // 这是我们要模拟的组名
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     fmt.Sprintf("wireflow-%s", roleName), // 绑定预定义的模板
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	err := w.client.Create(ctx, rb)

	return err
}
