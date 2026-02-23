package dto

import "wireflow/api/v1alpha1"

type PolicyDto struct {
	Name        string `json:"name"` // 只能是小写英文
	Namespace   string `json:"namespace"`
	Action      string `json:"action"` // Allow / Deny
	Description string `json:"description"`
	v1alpha1.WireflowPolicySpec
}
