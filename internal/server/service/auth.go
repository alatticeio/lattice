package service

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	serverutils "github.com/alatticeio/lattice/internal/server/utils"
	"github.com/alatticeio/lattice/pkg/utils"

	"gorm.io/gorm"
)

// AuthService handles token refresh and logout operations.
type AuthService interface {
	// RefreshAccessToken validates a refresh token, issues a new access token (15 min),
	// rotates the refresh token (old one revoked, new one issued), and returns both.
	RefreshAccessToken(ctx context.Context, refreshTokenRaw string) (accessToken string, newRefreshToken string, err error)

	// Logout revokes the given refresh token.
	Logout(ctx context.Context, refreshTokenRaw string) error
}

type authService struct {
	log   *log.Logger
	store store.Store
}

func NewAuthService(st store.Store) AuthService {
	return &authService{
		log:   log.GetLogger("auth-service"),
		store: st,
	}
}

func (s *authService) RefreshAccessToken(ctx context.Context, refreshTokenRaw string) (string, string, error) {
	hash := models.HashRefreshToken(refreshTokenRaw)

	// Look up the refresh token.
	rt, err := s.store.RefreshTokens().GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", errors.New("invalid refresh token")
		}
		return "", "", err
	}

	// Check if revoked
	if rt.RevokedAt != nil {
		return "", "", errors.New("refresh token has been revoked")
	}

	// Check if expired
	if time.Now().After(rt.ExpiresAt) {
		return "", "", errors.New("refresh token has expired")
	}

	// Look up user to get latest info
	user, err := s.store.Users().GetByID(ctx, rt.UserID)
	if err != nil {
		return "", "", errors.New("user not found")
	}

	// Check if user has been soft-deleted (disabled)
	if user.DeletedAt.Valid {
		return "", "", errors.New("user account is disabled")
	}

	now := time.Now()

	// Atomically revoke the old refresh token and create the new one.
	var rawToken string
	var accessToken string

	if err := s.store.Tx(ctx, func(tx store.Store) error {
		// Revoke the old refresh token
		if err := tx.RefreshTokens().Revoke(ctx, rt.ID, now); err != nil {
			return err
		}

		// Issue new access token (15 min)
		at, err := serverutils.GenerateBusinessJWTWithDuration(user.ID, user.Email, user.Username, string(user.SystemRole), 15*time.Minute)
		if err != nil {
			return err
		}
		accessToken = at

		// Issue new refresh token (30 days)
		raw := make([]byte, 32)
		if err := utils.GenerateRandomBytes(raw); err != nil {
			return err
		}
		rtRaw := base64.RawURLEncoding.EncodeToString(raw)
		rawToken = rtRaw

		newRT := &models.RefreshToken{
			UserID:    rt.UserID,
			TokenHash: models.HashRefreshToken(rtRaw),
			ExpiresAt: now.Add(720 * time.Hour), // 30 days
		}
		return tx.RefreshTokens().Create(ctx, newRT)
	}); err != nil {
		return "", "", err
	}

	return accessToken, rawToken, nil
}

func (s *authService) Logout(ctx context.Context, refreshTokenRaw string) error {
	hash := models.HashRefreshToken(refreshTokenRaw)
	rt, err := s.store.RefreshTokens().GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Silently succeed even if token is already gone
			return nil
		}
		return err
	}
	now := time.Now()
	return s.store.RefreshTokens().Revoke(ctx, rt.ID, now)
}
