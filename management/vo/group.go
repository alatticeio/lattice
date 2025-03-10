package vo

type GroupPolicyVo struct {
	ID          uint   `json:"id,string"`
	GroupId     uint   `json:"groupId,string"`
	PolicyId    uint   `json:"policyId,string"`
	PolicyName  string `json:"policyName"`
	Description string `json:"description"`
}
