package service

import (
	"context"
	"wireflow/api/v1alpha1"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/resource"
	"wireflow/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TokenService interface {
	Create(ctx context.Context) (string, error)
	Delete(ctx context.Context, token string) error
}

type tokenService struct {
	log           *log.Logger
	store         store.Store
	client        *resource.Client
	peerService   PeerService
	policyService PolicyService
}

func (t *tokenService) Delete(ctx context.Context, token string) error {
	wsId := ctx.Value(infra.WorkspaceKey).(string)
	workspace, err := t.store.Workspaces().GetByID(ctx, wsId)
	if err != nil {
		return err
	}

	res := &v1alpha1.WireflowEnrollmentToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      token,
			Namespace: workspace.Namespace,
		},
	}
	return client.IgnoreNotFound(t.client.Delete(ctx, res))
}

func (t *tokenService) Create(ctx context.Context) (string, error) {
	wsId := ctx.Value(infra.WorkspaceKey).(string)

	workspace, err := t.store.Workspaces().GetByID(ctx, wsId)
	if err != nil {
		return "", err
	}

	tokenStr, err := utils.GenerateSecureToken()
	if err != nil {
		return "", err
	}

	tokenDto := dto.TokenDto{
		Namespace: workspace.Namespace,
		Expiry:    "168h",
		Limit:     5,
		Name:      tokenStr,
	}

	if _, err = t.peerService.CreateToken(ctx, &tokenDto); err != nil {
		return "", err
	}

	if _, err := t.policyService.CreateOrUpdatePolicy(ctx, &dto.PolicyDto{
		Name:      "default-deny",
		Namespace: tokenDto.Namespace,
		Action:    "Deny",
	}); err != nil {
		return "", err
	}

	return tokenStr, nil
}

func NewTokenService(client *resource.Client, st store.Store) TokenService {
	return &tokenService{
		log:           log.GetLogger("token-service"),
		store:         st,
		peerService:   NewPeerService(client, st),
		policyService: NewPolicyService(client, st),
		client:        client,
	}
}
