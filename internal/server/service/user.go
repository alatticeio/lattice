package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/vo"
	"github.com/alatticeio/lattice/pkg/utils"

	"gorm.io/gorm"
)

type UserService interface {
	InitAdmin(ctx context.Context, admins []config.AdminConfig) error
	Register(ctx context.Context, userDto dto.UserDto) error
	Login(ctx context.Context, email, password string) (*models.User, error)
	GetMe(ctx context.Context, id string) (*models.User, error)
	List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error)

	OnboardExternalUser(ctx context.Context, provider, subject, email string, adminEmails []string) (*models.User, error)
	AddUser(ctx context.Context, dtos *dto.UserDto) error
	DeleteUser(ctx context.Context, username string) error
	UpdateSystemRole(ctx context.Context, userID string, role dto.SystemRole) error
}

type userService struct {
	log   *log.Logger
	store store.Store
}

func (u *userService) DeleteUser(ctx context.Context, id string) error {
	return u.store.Users().Delete(ctx, id)
}

func (u *userService) UpdateSystemRole(ctx context.Context, userID string, role dto.SystemRole) error {
	if role != dto.SystemRolePlatformAdmin && role != dto.SystemRoleUser {
		return errors.New("invalid system role")
	}
	user, err := u.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}
	user.SystemRole = role
	return u.store.Users().Update(ctx, user)
}

func (u *userService) AddUser(ctx context.Context, dto *dto.UserDto) error {
	return u.store.Tx(ctx, func(s store.Store) error {
		hashedPassword, err := utils.EncryptPassword(dto.Password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		newUser := &models.User{
			Username: dto.Username,
			Password: hashedPassword,
		}
		if err := s.Users().Create(ctx, newUser); err != nil {
			return err
		}
		ws, err := s.Workspaces().GetByNamespace(ctx, dto.Namespace)
		if err != nil {
			return err
		}
		return s.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
			Role:        dto.Role,
			Status:      "active",
			WorkspaceID: ws.ID,
			UserID:      newUser.ID,
		})
	})
}

func (u *userService) OnboardExternalUser(ctx context.Context, provider, subject, email string, adminEmails []string) (*models.User, error) {
	// Determine the role this email should get.
	targetRole := dto.SystemRoleUser
	for _, a := range adminEmails {
		if strings.EqualFold(a, email) {
			targetRole = dto.SystemRolePlatformAdmin
			break
		}
	}

	// Check if identity already exists.
	identity, err := u.store.UserIdentities().GetByProviderAndExternalID(ctx, provider, subject)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if identity != nil {
		user, err := u.store.Users().GetByID(ctx, identity.UserID)
		if err != nil {
			return nil, err
		}
		// Promote to admin if newly added to whitelist.
		if targetRole == dto.SystemRolePlatformAdmin && user.SystemRole != dto.SystemRolePlatformAdmin {
			user.SystemRole = dto.SystemRolePlatformAdmin
			if err := u.store.Users().Update(ctx, user); err != nil {
				u.log.Warn("failed to promote external user to platform_admin", "email", email, "err", err)
			}
		}
		return user, nil
	}

	// Look up user by email; create if not found.
	user, err := u.store.Users().GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if user == nil {
		user = &models.User{Email: email, SystemRole: targetRole}
		if err := u.store.Users().Create(ctx, user); err != nil {
			return nil, err
		}
	} else if targetRole == dto.SystemRolePlatformAdmin && user.SystemRole != dto.SystemRolePlatformAdmin {
		user.SystemRole = dto.SystemRolePlatformAdmin
		if err := u.store.Users().Update(ctx, user); err != nil {
			u.log.Warn("failed to promote existing user to platform_admin", "email", email, "err", err)
		}
	}

	// Create the identity link.
	if err := u.store.UserIdentities().Create(ctx, &models.UserIdentity{
		UserID:     user.ID,
		Provider:   provider,
		ExternalID: subject,
		Email:      email,
	}); err != nil {
		return nil, err
	}

	return user, nil
}

func (u *userService) List(ctx context.Context, req *dto.PageRequest) (*dto.PageResult[vo.UserVo], error) {
	users, total, err := u.store.Users().ListRaw(ctx, req)
	if err != nil {
		return nil, err
	}

	// Bulk-fetch accepted invitations to determine source/inviter.
	emails := make([]string, 0, len(users))
	for _, usr := range users {
		if usr.Email != "" {
			emails = append(emails, usr.Email)
		}
	}
	invitations, _ := u.store.WorkspaceInvitations().FindAcceptedByEmails(ctx, emails)
	// Build map email → earliest accepted invitation.
	invMap := make(map[string]*models.WorkspaceInvitation, len(invitations))
	for _, inv := range invitations {
		if _, exists := invMap[inv.Email]; !exists {
			invMap[inv.Email] = inv
		}
	}
	// Pre-fetch inviter usernames.
	inviterIDs := make([]string, 0)
	seen := map[string]bool{}
	for _, inv := range invMap {
		if inv.InviterID != "" && !seen[inv.InviterID] {
			inviterIDs = append(inviterIDs, inv.InviterID)
			seen[inv.InviterID] = true
		}
	}
	inviterNames := make(map[string]string, len(inviterIDs))
	for _, id := range inviterIDs {
		if inviter, err := u.store.Users().GetByID(ctx, id); err == nil {
			inviterNames[id] = inviter.Username
		}
	}

	vos := make([]vo.UserVo, 0, len(users))
	for _, usr := range users {
		source := "local"
		if len(usr.Identities) > 0 {
			source = usr.Identities[0].Provider
		}
		inviterName := ""
		if inv, ok := invMap[usr.Email]; ok {
			inviterName = inviterNames[inv.InviterID]
			source = "invitation"
		}
		vos = append(vos, vo.UserVo{
			ID:           usr.ID,
			Username:     usr.Username,
			Email:        usr.Email,
			Avatar:       usr.Avatar,
			Role:         string(usr.SystemRole),
			Source:       source,
			InviterName:  inviterName,
			RegisteredAt: usr.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return &dto.PageResult[vo.UserVo]{
		List:     vos,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (u *userService) InitAdmin(ctx context.Context, admins []config.AdminConfig) error {
	for _, admin := range admins {
		existing, err := u.store.Users().GetByUsername(ctx, admin.Username)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existing == nil {
			hashed, err := utils.EncryptPassword(admin.Password)
			if err != nil {
				u.log.Error("admin password hash failed", err, "username", admin.Username)
				continue
			}
			newUser := models.User{
				Username:   admin.Username,
				Password:   hashed,
				SystemRole: dto.SystemRolePlatformAdmin,
			}
			if err = u.store.Users().Create(ctx, &newUser); err != nil {
				u.log.Error("admin bootstrap failed", err, "username", admin.Username)
			} else {
				u.log.Info("admin account bootstrapped", "username", newUser.Username)
			}
		} else if existing.SystemRole != dto.SystemRolePlatformAdmin {
			existing.SystemRole = dto.SystemRolePlatformAdmin
			if err = u.store.Users().Update(ctx, existing); err != nil {
				u.log.Error("admin role update failed", err, "username", admin.Username)
			} else {
				u.log.Info("admin role updated to platform_admin", "username", existing.Username)
			}
		}
	}
	return nil
}

func (u *userService) GetMe(ctx context.Context, id string) (*models.User, error) {
	return u.store.Users().GetByID(ctx, id)
}

func (u *userService) Register(ctx context.Context, userDto dto.UserDto) error {
	password, err := utils.EncryptPassword(userDto.Password)
	if err != nil {
		return err
	}
	return u.store.Users().Create(ctx, &models.User{
		Username: userDto.Username,
		Password: password,
	})
}

func (u *userService) Login(ctx context.Context, username, password string) (*models.User, error) {
	user, err := u.store.Users().Login(ctx, username, password)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if err = utils.ComparePassword(user.Password, password); err != nil {
		return nil, errors.New("invalid credentials")
	}
	return user, nil
}

func NewUserService(st store.Store) UserService {
	return &userService{
		log:   log.GetLogger("user-service"),
		store: st,
	}
}
