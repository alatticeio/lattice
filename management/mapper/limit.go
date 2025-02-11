package mapper

import (
	"linkany/management/entity"
	"linkany/management/utils"
)

type UserConfigInterface interface {
	// Get returns user configs
	Get(token string) (*entity.UserConfig, error)

	Create(userConfig *entity.UserConfig) error

	Update(userConfig *entity.UserConfig) error

	Delete(userConfig *entity.UserConfig) error
}

type UserConfigMapper struct {
	*DatabaseService
	tokener    *utils.Tokener
	userMapper UserInterface
}

func NewUserConfigMapper(dataBaseService *DatabaseService) *UserConfigMapper {
	return &UserConfigMapper{DatabaseService: dataBaseService, tokener: utils.NewTokener(), userMapper: NewUserMapper(dataBaseService)}
}

func (ucm *UserConfigMapper) Get(token string) (*entity.UserConfig, error) {

	user, err := ucm.userMapper.Get(token)
	if err != nil {
		return nil, err
	}

	var userConfig entity.UserConfig
	if err := ucm.Where("user_id = ?", user.ID).First(&userConfig).Error; err != nil {
		return nil, err
	}
	return &userConfig, nil
}

func (ucm *UserConfigMapper) Create(userConfig *entity.UserConfig) error {
	return ucm.Model(ucm).Create(userConfig).Error
}

func (ucm *UserConfigMapper) Update(userConfig *entity.UserConfig) error {
	return ucm.Model(ucm).Save(userConfig).Error
}

func (ucm *UserConfigMapper) Delete(userConfig *entity.UserConfig) error {
	return ucm.Model(ucm).Delete(userConfig).Error
}
