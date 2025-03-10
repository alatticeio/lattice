package dto

type GroupPolicyDto struct {
	ID          uint   `json:"id,string"`
	GroupId     uint   `json:"groupId,string"`
	PolicyId    uint   `json:"policyId,string"`
	PolicyName  string `json:"policyName"`
	Description string `json:"description"`
}

type GroupPolicyParams struct {
	GroupId    uint   `json:"groupId" form:"groupId"`
	PolicyId   uint   `json:"policyId" form:"policyId"`
	PolicyName string `json:"policyName" form:"policyName"`
}
