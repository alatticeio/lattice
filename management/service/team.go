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

	client_r "wireflow/management/resource"

	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TeamService interface {
	OnboardExternalUser(ctx context.Context, userId, extEmail string) (*model.User, error)
	CreateNamespace(ctx context.Context, dto *dto.NamespaceDto) error
}

type teamService struct {
	log       *log.Logger
	K8sClient client.Client
	DB        *gorm.DB // SQLite 实例
	config    *config.Config
}

func NewTeamService(k8sClient *client_r.Client, config *config.Config) TeamService {
	return &teamService{
		log:       log.GetLogger("team-service"),
		K8sClient: k8sClient,
		DB:        database.DB,
		config:    config,
	}
}

func (t *teamService) OnboardExternalUser(ctx context.Context, externalID, email string) (*model.User, error) {
	var user model.User

	// 1. 先查数据库
	err := t.DB.Where("external_id = ? OR email = ?", externalID, email).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 2. 如果是新用户，判断是否应该给予 Admin 角色
		role := model.RoleViewer // 默认是普通用户
		for _, adminEmail := range t.config.App.InitAdmins {
			if email == adminEmail {
				role = model.RoleOwner
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
		if err := t.DB.Create(&user).Error; err != nil {
			return nil, err
		}

	}

	return &user, nil
}

// CreateTeamWithInfrastructure 创建 K8s Namespace、Quota、RoleBinding
func (t *teamService) CreateTeamWithInfrastructure(ctx context.Context, ownerID string, teamName string) error {
	teamID := fmt.Sprintf("wf-team-%t", ownerID[:8]) // 确保 ID 兼容 DNS 命名

	return t.DB.Transaction(func(tx *gorm.DB) error {
		// --- A. SQLite 事务 ---
		team := model.Team{DisplayName: teamName}

		if err := tx.Create(&team).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.TeamMember{TeamID: teamID, UserID: ownerID, Role: "admin"}).Error; err != nil {
			return err
		}

		// --- B. K8s 基础设施下发 ---

		// 1. 创建 Namespace
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   teamID,
			Labels: map[string]string{"managed-by": "wireflow"},
		}}
		if err := t.K8sClient.Create(ctx, ns); err != nil {
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
		if err := t.K8sClient.Create(ctx, quota); err != nil {
			return err
		}

		// 3. 创建 RoleBinding (将 Owner 绑定到 Namespace)
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "owner-binding", Namespace: teamID},
			Subjects: []rbacv1.Subject{{
				Kind:     "User",
				Name:     ownerID, // 这里对应外部身份系统的 ID/Email
				APIGroup: "rbac.authorization.k8s.io",
			}},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     "wf-resource-editor", // 引用之前定义的模板
				APIGroup: "rbac.authorization.k8s.io",
			},
		}
		return t.K8sClient.Create(ctx, rb)
	})
}

// teamService逻辑
func (t *teamService) CreateNamespace(ctx context.Context, dto *dto.NamespaceDto) error {

	// 1. 在 K8s 中真实创建 Namespace
	// 同时可以创建 ResourceQuota (配额), 限制 CPU/内存，防止用户把集群搞崩
	err := t.InitNewNamespace(ctx, dto)
	if err != nil {
		return err
	}

	// 2. 数据库记录这个 Namespace 及其归属信息
	newNS := model.Namespace{
		Name:        dto.Name,
		DisplayName: dto.DisplayName,
	}
	t.DB.Create(&newNS)

	return nil
}

// 授权逻辑
func (t *teamService) AssignPermission(ctx context.Context, dto *dto.UserNamespacePermissionDto) error {

	// 1. 数据库写入权限记录
	perm := &model.UserNamespacePermission{
		UserID:      dto.UserID,
		Namespace:   dto.Namespace,
		AccessLevel: dto.AccessLevel,
	}
	t.DB.Save(&perm)

	// 2. 【高级操作】同步创建 K8s RBAC RoleBinding
	// 这样该用户以后通过 kubectl 访问时也会被权限限制
	err := t.CreateRoleBinding(ctx, perm)
	if err != nil {
		return err
	}

	return nil
}

func (t *teamService) InitNewNamespace(ctx context.Context, dto *dto.NamespaceDto) error {
	return nil
}

func (t *teamService) CreateRoleBinding(ctx context.Context, perm *model.UserNamespacePermission) error {
	return nil
}
