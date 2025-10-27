package dto

import (
	"wireflow/management/utils"
	"wireflow/management/vo"
)

type UserParams struct {
	vo.PageModel
	Name string `json:"name" form:"name"`
}

func (up *UserParams) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if up.Name != "" {
		result = append(result, utils.NewKeyValue("name", up.Name))
	}

	return result
}

type UserResourcePermission struct {
	InviteId   uint64 `json:"inviteId" form:"inviteId"`
	ResourceId uint64 `json:"resourceId" form:"resourceId"`
}

func (urp *UserResourcePermission) Generate() []*utils.KeyValue {
	var result []*utils.KeyValue

	if urp.InviteId != 0 {
		result = append(result, utils.NewKeyValue("invite_id", urp.InviteId))
	}

	if urp.ResourceId != 0 {
		result = append(result, utils.NewKeyValue("resource_id", urp.ResourceId))
	}

	return result
}
