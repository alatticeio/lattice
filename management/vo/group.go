package vo

type GroupNodeVo struct {
	ModelVo
	GroupId   uint   `json:"groupId"`
	NodeId    uint   `gorm:"column:node_id;size:20" json:"nodeId"`
	GroupName string `gorm:"column:group_name;size:64" json:"groupName"`
	NodeName  string `gorm:"column:node_name;size:64" json:"nodeName"`
	CreatedBy string `gorm:"column:created_by;size:64" json:"createdBy"`
}

type NodeGroupVo struct {
	*GroupRelationVo
	ModelVo
	Name        string `json:"name"`
	NodeCount   int    `json:"nodeCount"`
	Status      string `json:"status"`
	Description string `json:"description"`
	CreatedBy   string `json:"createdBy"`
	UpdatedBy   string `json:"updatedBy"`

	GroupNodes    []GroupNodeVo   `json:"groupNodes"` // for tom-select show
	GroupPolicies []GroupPolicyVo `json:"groupPolicies"`
}

type GroupPolicyVo struct {
	ModelVo
	GroupId     uint   `json:"groupId,string"`
	PolicyId    uint   `json:"policyId,string"`
	PolicyName  string `json:"policyName"`
	Description string `json:"description"`
}
