package service

import (
	"context"
	"errors"
	"fmt"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/linkerrors"
	"linkany/pkg/redis"
	"strconv"
	"strings"
	"time"

	"github.com/pion/turn/v4"
	"gorm.io/gorm"
)

// UserService is an interface for user mapper
type UserService interface {
	Login(u *dto.UserDto) (*entity.Token, error)
	Register(e *dto.UserDto) (*entity.User, error)

	//Get returns a user by token
	Get(token string) (*entity.User, error)

	GetByUsername(username string) (*entity.User, error)
	QueryUsers(params *dto.UserParams) ([]*vo.UserVo, error)

	//Invite a user join network
	// Invite a user join network
	Invite(dto *dto.InviteDto) error
	CancelInvite(id string) error
	DeleteInvite(id string) error
	UpdateInvite(ctx context.Context, dto *dto.InviteDto) error
	GetInvite(ctx context.Context, id string) (*vo.InviteVo, error)
	GetInvitation(userId, email string) (*entity.Invitation, error)
	UpdateInvitation(dto *dto.InvitationDto) error
	RejectInvitation(id uint) error
	AcceptInvitation(id uint) error

	//ListInvitations list user invite from others
	ListInvitations(params *dto.InvitationParams) (*vo.PageVo, error)

	//listInvites user invite others list
	ListInvites(params *dto.InvitationParams) (*vo.PageVo, error)

	// User Permit
	//UserPermission grants a user permission to access a resource
	Permit(userID uint, resource string, accessLevel string) error

	//GetPermit fetches the permission details for a specific user and resource
	GetPermit(userID string, resource string) (*entity.UserPermission, error)

	//RevokePermit removes a user's permission to access a resource
	RevokePermit(userID string, resource string) error

	//ListPermits lists all permissions for a specific user
	ListPermits(userID string) ([]*entity.UserPermission, error)
}

var (
	_ UserService = (*userServiceImpl)(nil)
)

type userServiceImpl struct {
	*DatabaseService
	tokener *TokenService
	rdb     *redis.Client
}

func NewUserService(db *DatabaseService, rdb *redis.Client) UserService {
	return &userServiceImpl{DatabaseService: db, tokener: NewTokenService(dataBaseService), rdb: rdb}
}

// Login checks if the user exists and returns a token
func (u *userServiceImpl) Login(dto *dto.UserDto) (*entity.Token, error) {

	var user entity.User
	if err := u.Where("username = ?", dto.Username).First(&user).Error; err != nil {
		return nil, linkerrors.ErrUserNotFound
	}

	if err := utils.ComparePassword(user.Password, dto.Password); err != nil {
		return nil, linkerrors.ErrInvalidPassword
	}

	token, err := u.tokener.Generate(user.Username, user.Password)
	if err != nil {
		return nil, err
	}

	// Save turn key to redis
	key := turn.GenerateAuthKey(user.Username, "linkany.io", dto.Password)
	if err = u.rdb.Set(context.Background(), user.Username, string(key)); err != nil {
		return nil, err
	}
	return &entity.Token{Token: token, Avatar: user.Avatar, Email: user.Email, Mobile: user.Mobile}, nil
}

// Register creates a new user
func (u *userServiceImpl) Register(dto *dto.UserDto) (*entity.User, error) {
	hashedPassword, err := utils.EncryptPassword(dto.Password)
	if err != nil {
		return nil, err
	}
	e := &entity.User{
		Username: dto.Username,
		Password: hashedPassword,
	}
	err = u.Create(e).Error
	if err != nil {
		return nil, err
	}
	return e, nil
}

// Get returns a user by username
func (u *userServiceImpl) Get(token string) (*entity.User, error) {
	userToken, err := u.tokener.Parse(token)
	if err != nil {
		return nil, err
	}

	var user entity.User
	if err := u.Where("username = ?", userToken.Username).Find(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *userServiceImpl) GetByUsername(username string) (*entity.User, error) {
	var user entity.User
	if err := u.Where("username = ?", username).Find(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *userServiceImpl) QueryUsers(params *dto.UserParams) ([]*vo.UserVo, error) {

	var users []*entity.User
	result := new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := u.DB
	if sql != "" {
		db = u.Model(&entity.User{}).Where(sql, wrappers)
	}

	if err := db.Model(&entity.User{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := db.Model(&entity.User{}).Find(&users).Error; err != nil {
		return nil, err
	}

	var userVos []*vo.UserVo
	for _, user := range users {
		userVos = append(userVos, &vo.UserVo{
			ID:          user.ID,
			Username:    user.Username,
			Email:       user.Email,
			Avatar:      user.Avatar,
			MobilePhone: user.Mobile,
		})
	}

	result.Data = userVos
	return userVos, nil
}

// Invitation
func (u *userServiceImpl) Invite(dto *dto.InviteDto) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		var inviteUser, invitationUser *entity.User
		if inviteUser, err = u.GetByUsername(dto.InviterName); err != nil {
			return err
		}
		if invitationUser, err = u.GetByUsername(dto.InviteeName); err != nil {
			return err
		}

		// first query, if the invitation exists
		var exist entity.Invitation
		if err = tx.Model(&entity.Invitation{}).Where("invitee_name = ?", dto.InviteeName).First(&exist).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		if exist.ID != 0 {
			return linkerrors.ErrInvitationExists
		}

		groupName := getGroupNames(tx, dto.GroupIdList)
		invite := &entity.Invites{
			InvitationId: int64(invitationUser.ID),
			InviterId:    int64(inviteUser.ID),
			MobilePhone:  dto.MobilePhone,
			Email:        dto.Email,
			Group:        groupName,
			Permissions:  dto.Permissions,
			AcceptStatus: entity.NewInvite,
			InvitedAt:    time.Now(),
		}
		if err = tx.Create(invite).Error; err != nil {
			return err
		}

		if err = tx.Create(&entity.Invitation{
			InvitationId: invitationUser.ID,
			InviteeId:    inviteUser.ID,
			InviteId:     invite.ID,
			AcceptStatus: entity.NewInvite,
			Permissions:  dto.Permissions,
			Group:        groupName,
			Network:      dto.Network,
		}).Error; err != nil {
			return err
		}

		// insert into user granted permissions
		return addResourcePermission(tx, invite.ID, dto)
	})

}

func (u *userServiceImpl) UpdateInvite(ctx context.Context, dto *dto.InviteDto) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		groupName := getGroupNames(tx, dto.GroupIdList)

		if err = tx.Model(&entity.Invites{}).Where("id = ?", dto.ID).Updates(entity.Invites{
			Group:    groupName,
			GroupIds: dto.GroupIds,
		}).Error; err != nil {
			return err
		}

		if err = tx.Model(&entity.Invitation{}).Where("invite_id = ?", dto.ID).Updates(entity.Invitation{
			Group:    groupName,
			GroupIds: dto.GroupIds,
		}).Error; err != nil {
			return err
		}

		// insert into user granted permissions
		return updateResources(tx, dto.ID, dto)
	})
}

func updateResources(tx *gorm.DB, inviteId uint, dto *dto.InviteDto) error {
	var err error
	if err = tx.Model(&entity.SharedGroup{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedGroup{}).Error; err != nil {
		return err
	}

	if err = tx.Model(&entity.SharedNode{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedNode{}).Error; err != nil {
		return err
	}

	if err = tx.Model(&entity.SharedLabel{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedLabel{}).Error; err != nil {
		return err
	}

	if err = tx.Model(&entity.SharedPolicy{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedPolicy{}).Error; err != nil {
		return err
	}

	if err = tx.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ?", inviteId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
		return err
	}

	return addResourcePermission(tx, inviteId, dto)
}

func (u *userServiceImpl) GetInvite(ctx context.Context, id string) (*vo.InviteVo, error) {
	result := new(vo.InviteVo)
	var invite entity.Invites
	if err := u.Where("id = ?", id).First(&invite).Error; err != nil {
		return nil, err
	}

	var inviteUser, invitationUser entity.User
	if err := u.Where("id = ?", invite.InviterId).First(&inviteUser).Error; err != nil {
		return nil, err
	}

	if err := u.Where("id = ?", invite.InvitationId).First(&invitationUser).Error; err != nil {
		return nil, err
	}
	uvo, err := u.genUserResourceVo(invite.ID)
	if err != nil {
		return nil, err
	}

	result.UserResourceVo = uvo

	return result, nil
}

func (u *userServiceImpl) genUserResourceVo(inviteId uint) (*vo.UserResourceVo, error) {
	uvo := new(vo.UserResourceVo)
	// get invite resource info
	var sharedGroup []entity.SharedGroup
	if err := u.Model(&entity.SharedGroup{}).Where("invite_id = ?", inviteId).Find(&sharedGroup).Error; err != nil {
		return nil, err
	}

	if sharedGroup != nil {
		groupResourceVo := new(vo.GroupResourceVo)
		uvo.GroupResourceVo = groupResourceVo

		//var groupValues []*vo.ResourceValue
		groupValues := make(map[string]string, 1)
		for _, group := range sharedGroup {
			groupValues[fmt.Sprintf("%d", group.GroupId)] = group.GroupName
		}
		groupResourceVo.GroupValues = groupValues
	}

	// shared node
	var sharedNode []entity.SharedNode
	if err := u.Model(&entity.SharedNode{}).Where("invite_id = ?", inviteId).Find(&sharedNode).Error; err != nil {
		return nil, err
	}

	if sharedNode != nil {
		nodeResourceVo := new(vo.NodeResourceVo)
		uvo.NodeResourceVo = nodeResourceVo
		nodeValues := make(map[string]string, 1)
		for _, node := range sharedNode {
			nodeValues[fmt.Sprintf("%d", node.NodeId)] = node.NodeName
		}

		nodeResourceVo.NodeValues = nodeValues
	}

	// shared policy
	var sharedPolicy []entity.SharedPolicy
	if err := u.Model(&entity.SharedPolicy{}).Where("invite_id = ?", inviteId).Find(&sharedPolicy).Error; err != nil {
		return nil, err
	}

	if sharedPolicy != nil {
		policyResourceVo := new(vo.PolicyResourceVo)
		uvo.PolicyResourceVo = policyResourceVo
		policyValues := make(map[string]string, 1)
		for _, policy := range sharedPolicy {
			policyValues[fmt.Sprintf("%d", policy.PolicyId)] = policy.PolicyName
		}

		policyResourceVo.PolicyValues = policyValues
	}

	// share label
	var sharedLabel []entity.SharedLabel
	if err := u.Model(&entity.SharedLabel{}).Where("invite_id = ?", inviteId).Find(&sharedLabel).Error; err != nil {
		return nil, err
	}

	if sharedLabel != nil {
		labelResourceVo := new(vo.LabelResourceVo)
		uvo.LabelResourceVo = labelResourceVo
		labelValues := make(map[string]string, 1)
		for _, label := range sharedLabel {
			labelValues[fmt.Sprintf("%d", label.LabelId)] = label.LabelName
		}

		labelResourceVo.LabelValues = labelValues
	}

	// shared permissions
	var sharedPermissions []entity.UserResourceGrantedPermission
	if err := u.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ?", inviteId).Find(&sharedPermissions).Error; err != nil {
		return nil, err
	}

	if sharedPermissions != nil {
		permissionResourceVo := new(vo.PermissionResourceVo)
		uvo.PermissionResourceVo = permissionResourceVo
		permissionValues := make(map[string]string, 1)
		for _, sharedPermission := range sharedPermissions {
			if (permissionValues[fmt.Sprintf("%d", sharedPermission.PermissionId)]) != "" {
				continue
			}
			permissionValues[fmt.Sprintf("%d", sharedPermission.PermissionId)] = sharedPermission.PermissionText
		}

		permissionResourceVo.PermissionValues = permissionValues
	}

	return uvo, nil
}

func addResourcePermission(tx *gorm.DB, inviteId uint, dto *dto.InviteDto) error {

	var (
		allNames []string
	)

	if dto.PolicyIdList != nil {
		names, values, ids, err := getActualPermission(tx, utils.Policy, dto)
		if err != nil {
			return err
		}
		allNames = append(allNames, names...)

		var policies []entity.Node
		if err = tx.Model(&entity.Node{}).Where("id in ?", dto.GroupIdList).Find(&policies).Error; err != nil {
			return err
		}

		for _, policy := range policies {

			// insert into shared policy
			sharedPolicy := &entity.SharedPolicy{
				OwnerId:      uint(dto.InviteeId),
				UserId:       uint(dto.InvitationId),
				InviteId:     inviteId,
				AcceptStatus: entity.NewInvite,
				PolicyId:     policy.ID,
				PolicyName:   policy.Name,
				GrantedAt:    utils.NewNullTime(time.Now()),
			}

			if err = tx.Model(&entity.SharedPolicy{}).Create(sharedPolicy).Error; err != nil {
				return err
			}

			if err = createResourcePermission(&createPermissonParams{
				Tx:               tx,
				ResourceType:     utils.Policy,
				OwnerId:          sharedPolicy.OwnerId,
				InvitationId:     sharedPolicy.UserId,
				InviteId:         inviteId,
				ResourceId:       sharedPolicy.ID,
				PermissionTexts:  names,
				PermissionValues: values,
				PermissionIds:    ids,
			}); err != nil {
				return err
			}
		}
	}

	if dto.NodeIdList != nil {
		names, values, ids, err := getActualPermission(tx, utils.Node, dto)
		if err != nil {
			return err
		}
		allNames = append(allNames, names...)

		var nodes []entity.Node
		if err = tx.Model(&entity.Node{}).Where("id in ?", dto.NodeIdList).Find(&nodes).Error; err != nil {
			return err
		}

		for _, node := range nodes {
			// insert into shared node
			sharedNode := &entity.SharedNode{
				OwnerId:      uint(dto.InviteeId),
				UserId:       uint(dto.InvitationId),
				InviteId:     inviteId,
				AcceptStatus: entity.NewInvite,
				NodeId:       node.ID,
				NodeName:     node.Name,
				GrantedAt:    utils.NewNullTime(time.Now()),
			}

			if err = tx.Model(&entity.SharedNode{}).Create(sharedNode).Error; err != nil {
				return err
			}

			if err = createResourcePermission(&createPermissonParams{
				Tx:               tx,
				ResourceType:     utils.Node,
				OwnerId:          sharedNode.OwnerId,
				InvitationId:     sharedNode.UserId,
				InviteId:         inviteId,
				ResourceId:       sharedNode.ID,
				PermissionTexts:  names,
				PermissionValues: values,
				PermissionIds:    ids,
			}); err != nil {
				return err
			}
		}
	}

	if dto.LabelIdList != nil {
		names, values, ids, err := getActualPermission(tx, utils.Label, dto)
		if err != nil {
			return err
		}
		allNames = append(allNames, names...)

		var labels []entity.Label
		if err = tx.Model(&entity.Label{}).Where("id in ?", dto.NodeIdList).Find(&labels).Error; err != nil {
			return err
		}

		for _, label := range labels {

			// insert into shared label
			sharedLabel := &entity.SharedLabel{
				OwnerId:      uint(dto.InviteeId),
				UserId:       uint(dto.InvitationId),
				InviteId:     inviteId,
				AcceptStatus: entity.NewInvite,
				LabelId:      label.ID,
				LabelName:    label.Label,
				GrantedAt:    utils.NewNullTime(time.Now()),
			}

			if err = tx.Model(&entity.SharedLabel{}).Create(sharedLabel).Error; err != nil {
				return err
			}

			if err = createResourcePermission(&createPermissonParams{
				Tx:               tx,
				ResourceType:     utils.Label,
				OwnerId:          sharedLabel.OwnerId,
				InvitationId:     sharedLabel.UserId,
				InviteId:         inviteId,
				ResourceId:       sharedLabel.ID,
				PermissionTexts:  names,
				PermissionValues: values,
				PermissionIds:    ids,
			}); err != nil {
				return err
			}
		}
	}

	if dto.GroupIdList != nil {
		names, values, ids, err := getActualPermission(tx, utils.Group, dto)
		if err != nil {
			return err
		}
		allNames = append(allNames, names...)

		var groups []entity.NodeGroup
		if err = tx.Model(&entity.NodeGroup{}).Where("id in ?", dto.GroupIdList).Find(&groups).Error; err != nil {
			return err
		}

		for _, group := range groups {
			// insert into shared group
			sharedGroup := &entity.SharedGroup{
				OwnerId:      uint(dto.InviteeId),
				UserId:       uint(dto.InvitationId),
				GroupName:    group.Name,
				InviteId:     inviteId,
				AcceptStatus: entity.NewInvite,
				GroupId:      group.ID,
				GrantedAt:    utils.NewNullTime(time.Now()),
			}

			if err = tx.Model(&entity.SharedGroup{}).Create(sharedGroup).Error; err != nil {
				return err
			}

			if err = createResourcePermission(&createPermissonParams{
				Tx:               tx,
				ResourceType:     utils.Group,
				OwnerId:          sharedGroup.OwnerId,
				InvitationId:     sharedGroup.UserId,
				InviteId:         inviteId,
				ResourceId:       sharedGroup.ID,
				PermissionTexts:  names,
				PermissionValues: values,
				PermissionIds:    ids,
			}); err != nil {
				return err
			}
		}
	}

	// update invite permissinos
	if err := tx.Model(&entity.Invites{}).Where("id = ?", inviteId).Update("permissions", strings.Join(allNames, ",")).Error; err != nil {
		return err
	}

	return nil
}

type createPermissonParams struct {
	Tx               *gorm.DB
	ResourceType     utils.ResourceType
	OwnerId          uint
	InvitationId     uint
	InviteId         uint
	ResourceId       uint
	PermissionTexts  []string
	PermissionValues []string
	PermissionIds    []uint
}

func createResourcePermission(params *createPermissonParams) error {
	for i, permissionText := range params.PermissionTexts {
		permit := &entity.UserResourceGrantedPermission{
			OwnerId:         params.OwnerId,
			InvitationId:    params.InvitationId,
			ResourceType:    params.ResourceType,
			ResourceId:      params.ResourceId,
			InviteId:        params.InviteId,
			PermissionText:  permissionText,
			PermissionValue: params.PermissionValues[i],
			PermissionId:    params.PermissionIds[i],
		}
		if err := params.Tx.Create(permit).Error; err != nil {
			return err
		}
	}

	return nil

}

// getActualPermission return names, values, ids, err
func getActualPermission(tx *gorm.DB, resType utils.ResourceType, dto *dto.InviteDto) ([]string, []string, []uint, error) {
	var permissions []entity.Permissions
	if err := tx.Model(&entity.Permissions{}).Where("id in ? and permission_type = ?", dto.PermissionIdList, resType).Find(&permissions).Error; err != nil {
		return nil, nil, nil, err
	}

	var names []string
	var ids []uint
	var values []string
	for _, permission := range permissions {
		names = append(names, permission.Name)
		ids = append(ids, permission.ID)
		values = append(values, permission.PermissionValue)
	}

	return names, values, ids, nil
}

func (u *userServiceImpl) CancelInvite(id string) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		//delete role &  permissions

		var invite entity.Invites

		var err error
		if err = tx.Model(&entity.Invites{}).Where("id = ?", id).Find(&invite).Update("accept_status", entity.Canceled).Error; err != nil {
			return err
		}

		return updateResourcePermission(tx, invite.ID, entity.Canceled)

	})
}

func (u *userServiceImpl) DeleteInvite(id string) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		//delete role &  permissions

		var invite entity.Invites

		var err error
		if err = tx.Model(&entity.Invites{}).Where("id = ?", id).Find(&invite).Delete(&entity.Invites{}).Error; err != nil {
			return err
		}

		return deleteResourcePermission(tx, invite.ID)

	})
}

func getGroupNames(tx *gorm.DB, ids []uint) string {
	var result []string
	for _, id := range ids {
		var group entity.NodeGroup
		if err := tx.Where("id = ?", id).First(&group).Error; err != nil {
			return ""
		}
		result = append(result, group.Name)
	}

	return utils.Join(result, ",")
}

func (u *userServiceImpl) GetInvitation(userId, email string) (*entity.Invitation, error) {
	var inv entity.Invitation
	if err := u.Where("invitation_id = ? AND email = ?", userId, email).First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

func (u *userServiceImpl) UpdateInvitation(dto *dto.InvitationDto) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		var (
			inv entity.Invitation
			err error
		)
		if err = tx.Model(&entity.Invitation{}).Where("id = ?", dto.ID).Find(&inv).Update("accept_status", dto.AcceptStatus).Error; err != nil {
			return err
		}

		// if reject, return
		if dto.AcceptStatus == entity.Rejected {
			return nil
		}
		// data insert to shared
		groupIds := strings.Split(inv.GroupIds, ",")
		for _, groupId := range groupIds {
			gid, err := strconv.Atoi(groupId)
			if err != nil {
				return errors.New("invalid groupId")
			}
			shareGroup := &entity.UserGroupShared{
				OwnerId:     inv.InviteeId,
				UserId:      inv.InvitationId,
				GroupId:     uint(gid),
				Description: "",
			}

			if err = tx.Model(&entity.UserGroupShared{}).Create(shareGroup).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (u *userServiceImpl) RejectInvitation(id uint) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		var inv entity.Invitation
		var err error
		if err = tx.Model(&entity.Invitation{}).Where("invite_id = ?", id).Find(&inv).Update("accept_status", entity.Rejected).Error; err != nil {
			return err
		}
		return updateResourcePermission(tx, id, entity.Rejected)
	})
}

func (u *userServiceImpl) AcceptInvitation(id uint) error {
	return u.DB.Transaction(func(tx *gorm.DB) error {
		var inv entity.Invitation
		var err error
		if err = tx.Model(&entity.Invitation{}).Where("invite_id = ?", id).Find(&inv).Update("accept_status", entity.Accept).Error; err != nil {
			return err
		}

		// update shared and permissions table
		return updateResourcePermission(tx, id, entity.Accept)
	})
}

func updateResourcePermission(tx *gorm.DB, inviteId uint, status entity.AcceptStatus) error {
	// update shared group
	var (
		err error
	)
	if err = tx.Model(&entity.SharedGroup{}).Where("invite_id = ?", inviteId).Update("accept_status", status).Error; err != nil {
		return err
	}

	// update shared node
	if err = tx.Model(&entity.SharedNode{}).Where("invite_id = ?", inviteId).Update("accept_status", status).Error; err != nil {
		return err
	}

	// update shared label
	if err = tx.Model(&entity.SharedLabel{}).Where("invite_id = ?", inviteId).Update("accept_status", status).Error; err != nil {
		return err
	}

	// update shared policy
	if err = tx.Model(&entity.SharedPolicy{}).Where("invite_id = ?", inviteId).Update("accept_status", status).Error; err != nil {
		return err
	}

	// update shared perissions
	if err = tx.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ?", inviteId).Update("accept_status", status).Error; err != nil {
		return err
	}

	switch status {
	case entity.Canceled:
		if err = tx.Model(&entity.Invitation{}).Where("invite_id = ?", inviteId).Update("accept_status", entity.Canceled).Error; err != nil {
			return err
		}
	default:
		// update invite table
		if err = tx.Model(&entity.Invites{}).Where("id = ?", inviteId).Update("accept_status", status).Error; err != nil {
			return err
		}
	}

	return nil
}

func deleteResourcePermission(tx *gorm.DB, inviteId uint) error {
	// update shared group
	var (
		err error
	)
	if err = tx.Model(&entity.SharedGroup{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedGroup{}).Error; err != nil {
		return err
	}

	// update shared node
	if err = tx.Model(&entity.SharedNode{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedNode{}).Error; err != nil {
		return err
	}

	// update shared label
	if err = tx.Model(&entity.SharedLabel{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedLabel{}).Error; err != nil {
		return err
	}

	// update shared policy
	if err = tx.Model(&entity.SharedPolicy{}).Where("invite_id = ?", inviteId).Delete(&entity.SharedPolicy{}).Error; err != nil {
		return err
	}

	// update shared perissions
	if err = tx.Model(&entity.UserResourceGrantedPermission{}).Where("invite_id = ?", inviteId).Delete(&entity.UserResourceGrantedPermission{}).Error; err != nil {
		return err
	}
	return nil
}

func (u *userServiceImpl) ListInvites(params *dto.InvitationParams) (*vo.PageVo, error) {

	var invs []*entity.Invites
	result := new(vo.PageVo)
	sql, wrappers := utils.Generate(params)
	db := u.DB
	if sql != "" {
		db = u.Model(&entity.Invites{}).Where(sql, wrappers)
	}

	if err := db.Model(&entity.Invites{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := db.Model(&entity.Invites{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&invs).Error; err != nil {
		return nil, err
	}

	var insVos []*vo.InviteVo
	for _, inv := range invs {
		var inviteUser entity.User
		var invitationUser entity.User
		var err error
		if err = db.Model(&entity.User{}).Where("id = ?", inv.InviterId).First(&inviteUser).Error; err != nil {
			return nil, err
		}

		if err = db.Model(&entity.User{}).Where("id = ?", inv.InvitationId).First(&invitationUser).Error; err != nil {
			return nil, err
		}

		uvo, err := u.genUserResourceVo(inv.ID)
		if err != nil {
			return nil, err
		}

		insVo := &vo.InviteVo{
			UserResourceVo: uvo,
			ID:             uint64(inv.ID),
			InviteeName:    invitationUser.Username,
			InviterName:    inviteUser.Username,
			MobilePhone:    invitationUser.Mobile,
			Email:          invitationUser.Email,
			Avatar:         invitationUser.Avatar,
			Role:           inv.Role,
			GroupName:      inv.Group,
			Permissions:    inv.Permissions,
			AcceptStatus:   inv.AcceptStatus.String(),
			InvitedAt:      inv.InvitedAt,
		}

		insVos = append(insVos, insVo)
	}

	result.Data = insVos
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size
	return result, nil
}

func (u *userServiceImpl) ListInvitations(params *dto.InvitationParams) (*vo.PageVo, error) {
	var invs []*entity.Invitation
	result := new(vo.PageVo)
	db := u.DB
	sql, wrappers := utils.Generate(params)
	if sql != "" {
		db = u.Model(&entity.Invitation{}).Where(sql, wrappers)
	}

	if err := db.Model(&entity.Invitation{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	if err := u.Model(&entity.Invitation{}).Offset((params.Page - 1) * params.Size).Limit(params.Size).Find(&invs).Error; err != nil {
		return nil, err
	}

	var insVos []*vo.InvitationVo
	for _, inv := range invs {
		var inviteUser entity.User
		var err error
		if err = db.Model(&entity.User{}).Where("id = ?", inv.InviteeId).First(&inviteUser).Error; err != nil {
			return nil, err
		}

		insVo := &vo.InvitationVo{
			ID:            uint64(inv.ID),
			Group:         inv.Group,
			InviterName:   inviteUser.Username,
			InviterAvatar: inviteUser.Avatar,
			InviteId:      inv.InviteId,
			Role:          inv.Role,
			AcceptStatus:  inv.AcceptStatus.String(),
			Permissions:   inv.Permissions,

			InvitedAt: inv.InvitedAt,
		}

		insVos = append(insVos, insVo)
	}

	result.Data = insVos
	result.Current = params.Page
	result.Page = params.Page
	result.Size = params.Size

	return result, nil
}

func (u *userServiceImpl) Permit(userID uint, resource string, permission string) error {
	//TODO Get user's permissions first, if nil, add, else update

	permit := entity.UserPermission{
		UserID:       userID,
		ResourceType: resource,
		Permissions:  permission,
	}
	if err := u.Create(&permit).Error; err != nil {
		return err
	}
	return nil
}

func (u *userServiceImpl) GetPermit(userID string, resource string) (*entity.UserPermission, error) {
	var permit entity.UserPermission
	if err := u.Where("user_id = ? AND resource = ?", userID, resource).First(&permit).Error; err != nil {
		return nil, err
	}
	return &permit, nil
}

func (u *userServiceImpl) RevokePermit(userID string, resource string) error {
	if err := u.Where("user_id = ? AND resource = ?", userID, resource).Delete(&entity.UserPermission{}).Error; err != nil {
		return err
	}
	return nil
}

func (u *userServiceImpl) ListPermits(userID string) ([]*entity.UserPermission, error) {
	var permits []*entity.UserPermission
	if err := u.Where("user_id = ?", userID).Find(&permits).Error; err != nil {
		return nil, err
	}
	return permits, nil
}
