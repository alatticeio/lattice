package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
	"wireflow/management/vo"
	"wireflow/pkg/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InvitationService manages workspace invitations.
type InvitationService interface {
	Create(ctx context.Context, workspaceID, inviterID, email string, role dto.WorkspaceRole) (*models.WorkspaceInvitation, error)
	// Preview returns public invitation details for display before login/register.
	Preview(ctx context.Context, token string) (*vo.InvitePreviewVo, error)
	// Accept adds the acceptor as a workspace member. Validates email match.
	Accept(ctx context.Context, token, acceptorUserID string) error
	// RegisterAndAccept creates a new user account then immediately accepts the invitation.
	RegisterAndAccept(ctx context.Context, token, username, password string) (string, error)
	Revoke(ctx context.Context, callerID, invitationID string) error
	List(ctx context.Context, workspaceID string) ([]*models.WorkspaceInvitation, error)
}

type invitationService struct {
	store store.Store
}

func NewInvitationService(st store.Store) InvitationService {
	return &invitationService{store: st}
}

func (s *invitationService) Create(ctx context.Context, workspaceID, inviterID, email string, role dto.WorkspaceRole) (*models.WorkspaceInvitation, error) {
	// Platform admins can invite to any workspace regardless of membership.
	inviter, err := s.store.Users().GetByID(ctx, inviterID)
	if err != nil {
		return nil, errors.New("inviter not found")
	}
	isPlatformAdmin := inviter.SystemRole == dto.SystemRolePlatformAdmin

	if !isPlatformAdmin {
		inviterMember, err := s.store.WorkspaceMembers().GetMembership(ctx, workspaceID, inviterID)
		if err != nil {
			return nil, errors.New("inviter is not a member of this workspace")
		}
		if dto.GetRoleWeight(inviterMember.Role) < dto.GetRoleWeight(dto.RoleAdmin) {
			return nil, errors.New("only admins can invite members")
		}
		if dto.GetRoleWeight(role) > dto.GetRoleWeight(inviterMember.Role) {
			return nil, errors.New("cannot invite with a role higher than your own")
		}
	}

	// Prevent duplicate pending invitations.
	existing, err := s.store.WorkspaceInvitations().GetPendingByEmailAndWorkspace(ctx, email, workspaceID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("a pending invitation already exists for this email")
	}

	token, err := generateInviteToken()
	if err != nil {
		return nil, err
	}

	inv := &models.WorkspaceInvitation{
		WorkspaceID: workspaceID,
		InviterID:   inviterID,
		Email:       email,
		Role:        role,
		Token:       token,
		Status:      "pending",
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}
	if err := s.store.WorkspaceInvitations().Create(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (s *invitationService) Preview(ctx context.Context, token string) (*vo.InvitePreviewVo, error) {
	inv, err := s.store.WorkspaceInvitations().GetByToken(ctx, token)
	if err != nil {
		return nil, errors.New("invitation not found")
	}

	ws, err := s.store.Workspaces().GetByID(ctx, inv.WorkspaceID)
	if err != nil {
		return nil, errors.New("workspace not found")
	}

	preview := &vo.InvitePreviewVo{
		Email:         inv.Email,
		WorkspaceID:   inv.WorkspaceID,
		WorkspaceName: ws.DisplayName,
		Role:          string(inv.Role),
		ExpiresAt:     inv.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		Status:        inv.Status,
	}

	// Best-effort: enrich with inviter info.
	if inviter, err := s.store.Users().GetByID(ctx, inv.InviterID); err == nil {
		preview.InviterName = inviter.Username
		preview.InviterEmail = inviter.Email
	}

	return preview, nil
}

func (s *invitationService) Accept(ctx context.Context, token, acceptorUserID string) error {
	return s.store.Tx(ctx, func(st store.Store) error {
		inv, err := st.WorkspaceInvitations().GetByToken(ctx, token)
		if err != nil {
			return errors.New("invitation not found")
		}
		if inv.Status != "pending" {
			return errors.New("invitation is no longer pending")
		}
		if time.Now().After(inv.ExpiresAt) {
			_ = st.WorkspaceInvitations().UpdateStatus(ctx, inv.ID, "expired")
			return errors.New("invitation has expired")
		}

		// Validate email: the logged-in user must match the invitation email.
		acceptor, err := st.Users().GetByID(ctx, acceptorUserID)
		if err != nil {
			return errors.New("user not found")
		}
		if !strings.EqualFold(acceptor.Email, inv.Email) {
			return fmt.Errorf("this invitation was sent to %s, but you are logged in as %s", inv.Email, acceptor.Email)
		}

		now := time.Now()
		if err := st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
			WorkspaceID: inv.WorkspaceID,
			UserID:      acceptorUserID,
			Role:        inv.Role,
			Status:      "active",
			InvitedBy:   inv.InviterID,
			JoinedAt:    &now,
		}); err != nil {
			return err
		}
		return st.WorkspaceInvitations().UpdateStatus(ctx, inv.ID, "accepted")
	})
}

// RegisterAndAccept creates a new User with the invitation email, then accepts the invitation.
// Returns a signed JWT on success.
func (s *invitationService) RegisterAndAccept(ctx context.Context, token, username, password string) (string, error) {
	var jwtToken string

	err := s.store.Tx(ctx, func(st store.Store) error {
		inv, err := st.WorkspaceInvitations().GetByToken(ctx, token)
		if err != nil {
			return errors.New("invitation not found")
		}
		if inv.Status != "pending" {
			return errors.New("invitation is no longer pending")
		}
		if time.Now().After(inv.ExpiresAt) {
			_ = st.WorkspaceInvitations().UpdateStatus(ctx, inv.ID, "expired")
			return errors.New("invitation has expired")
		}

		// Check if email already registered.
		existing, err := st.Users().GetByEmail(ctx, inv.Email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existing != nil {
			return errors.New("an account with this email already exists, please log in instead")
		}

		// Check username uniqueness.
		if _, err := st.Users().GetByUsername(ctx, username); err == nil {
			return errors.New("username already taken")
		}

		hashed, err := utils.EncryptPassword(password)
		if err != nil {
			return err
		}

		user := &models.User{
			Model:    models.Model{ID: uuid.New().String()},
			Username: username,
			Email:    inv.Email,
			Password: hashed,
		}
		if err := st.Users().Create(ctx, user); err != nil {
			return err
		}

		now := time.Now()
		if err := st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
			WorkspaceID: inv.WorkspaceID,
			UserID:      user.ID,
			Role:        inv.Role,
			Status:      "active",
			InvitedBy:   inv.InviterID,
			JoinedAt:    &now,
		}); err != nil {
			return err
		}

		if err := st.WorkspaceInvitations().UpdateStatus(ctx, inv.ID, "accepted"); err != nil {
			return err
		}

		jwtToken, err = utils.GenerateBusinessJWT(user.ID, user.Email, user.Username, string(user.SystemRole))
		return err
	})

	return jwtToken, err
}

func (s *invitationService) Revoke(ctx context.Context, callerID, invitationID string) error {
	inv, err := s.store.WorkspaceInvitations().FindByID(ctx, invitationID)
	if err != nil {
		return errors.New("invitation not found")
	}

	caller, err := s.store.Users().GetByID(ctx, callerID)
	if err != nil {
		return errors.New("caller not found")
	}

	// Platform admins can revoke any invitation.
	if caller.SystemRole != dto.SystemRolePlatformAdmin {
		// Workspace admins and the original inviter can revoke.
		isInviter := inv.InviterID == callerID
		if !isInviter {
			member, err := s.store.WorkspaceMembers().GetMembership(ctx, inv.WorkspaceID, callerID)
			if err != nil || dto.GetRoleWeight(member.Role) < dto.GetRoleWeight(dto.RoleAdmin) {
				return errors.New("permission denied: only the inviter or a workspace admin can revoke this invitation")
			}
		}
	}

	return s.store.WorkspaceInvitations().UpdateStatus(ctx, invitationID, "revoked")
}

func (s *invitationService) List(ctx context.Context, workspaceID string) ([]*models.WorkspaceInvitation, error) {
	return s.store.WorkspaceInvitations().ListByWorkspace(ctx, workspaceID)
}

func generateInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
