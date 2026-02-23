package service

import (
	"context"
	"strings"
	"wireflow/api/v1alpha1"
	"wireflow/internal/log"
	"wireflow/management/database"
	"wireflow/management/dto"
	"wireflow/management/repository"
	"wireflow/management/resource"
	"wireflow/pkg/utils"

	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TokenService interface {
	Create(ctx context.Context) error
	Delete(ctx context.Context, token string) error
}

type tokenService struct {
	log *log.Logger
	db  *gorm.DB

	client        *resource.Client
	peerService   PeerService
	policyService PolicyService
	workspaceRepo repository.WorkspaceRepository
}

func (t tokenService) Delete(ctx context.Context, token string) error {
	return t.db.Transaction(func(tx *gorm.DB) error {
		wsId := ctx.Value("workspaceId").(string)
		workspaceRepo := t.workspaceRepo.WithTx(tx)
		workspace, err := workspaceRepo.FindById(ctx, wsId)
		if err != nil {
			return err
		}

		resource := &v1alpha1.WireflowEnrollmentToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      token,
				Namespace: workspace.Namespace,
			},
		}

		if err := t.client.Delete(ctx, resource); err != nil {
			return client.IgnoreNotFound(err) // 如果资源已经被删除了，忽略报错
		}

		return nil
	})

}

func (t tokenService) Create(ctx context.Context) error {
	wsId := ctx.Value("workspaceId").(string)
	return t.db.Transaction(func(tx *gorm.DB) error {
		workspaceRepo := t.workspaceRepo.WithTx(tx)
		workspace, err := workspaceRepo.FindById(ctx, wsId)
		if err != nil {
			return err
		}

		tokenDto := dto.TokenDto{
			Namespace: workspace.Namespace,
			Expiry:    "168h",
			Limit:     5,
		}

		tokenStr, err := utils.GenerateSecureToken()
		if err != nil {
			return err
		}

		tokenDto.Name = tokenStr

		_, err = t.peerService.CreateToken(ctx, &tokenDto)
		if err != nil {
			return err
		}

		// create default deny
		if _, err := t.policyService.CreateOrUpdatePolicy(ctx, &dto.PolicyDto{
			Name:      strings.ToLower(tokenDto.Name),
			Namespace: tokenDto.Namespace,
			Action:    "Deny",
		}); err != nil {
			return err
		}

		return nil
	})
}

func NewTokenService(client *resource.Client) TokenService {
	return &tokenService{
		log:           log.GetLogger("user-service"),
		db:            database.DB,
		workspaceRepo: repository.NewWorkspaceRepository(),
		peerService:   NewPeerService(client),
		policyService: NewPolicyService(client),
		client:        client,
	}
}
