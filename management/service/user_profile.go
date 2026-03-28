package service

import (
	"context"
	"wireflow/internal/store"
	"wireflow/management/dto"
	"wireflow/management/models"
)

// ProfileService 定义个人信息业务接口
type ProfileService interface {
	GetProfile(ctx context.Context, userID string) (*dto.UserSettingsResponse, error)
	UpdateProfile(ctx context.Context, userID string, req dto.UpdateSettingsRequest) error
}

type profileService struct {
	store store.Store
}

func NewProfileService(st store.Store) ProfileService {
	return &profileService{store: st}
}

func (s *profileService) GetProfile(ctx context.Context, userID string) (*dto.UserSettingsResponse, error) {
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile, err := s.store.Profiles().Get(ctx, userID)
	if err != nil {
		// 不存在时返回空 profile，不阻断请求
		profile = &models.UserProfile{}
	}

	return &dto.UserSettingsResponse{
		Name:        user.Username,
		Email:       user.Email,
		AvatarURL:   user.Avatar,
		Title:       profile.Title,
		Company:     profile.Company,
		Bio:         profile.Bio,
		Timezone:    profile.Timezone,
		Language:    profile.Language,
		EmailNotify: profile.EmailNotify,
	}, nil
}

func (s *profileService) UpdateProfile(ctx context.Context, userID string, req dto.UpdateSettingsRequest) error {
	return s.store.Tx(ctx, func(tx store.Store) error {
		user := &models.User{
			Email:   req.Email,
			Avatar:  req.AvatarURL,
			Address: req.Address,
			Gender:  req.Gender,
		}
		user.ID = userID
		if err := tx.Users().Update(ctx, user); err != nil {
			return err
		}
		return tx.Profiles().Upsert(ctx, &models.UserProfile{
			UserID:      userID,
			Title:       req.Title,
			Company:     req.Company,
			Bio:         req.Bio,
			Timezone:    req.Timezone,
			Language:    req.Language,
			EmailNotify: req.EmailNotify,
		})
	})
}
