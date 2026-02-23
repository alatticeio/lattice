package vo

import (
	"wireflow/api/v1alpha1"
)

type PolicyVo struct {
	// 基础元数据
	Name        string `json:"name" binding:"required,lowercase"`
	Action      string `json:"action" binding:"oneof=Allow Deny"`
	Description string `json:"description"`
	Namespace   string `json:"namespace"`

	PolicyTypes []string `json:"policyTypes"`
	// 策略核心：使用指针嵌套
	// 这样前端传参可以扁平，也可以通过判断 nil 知道用户是否传了策略部分
	*v1alpha1.WireflowPolicySpec `json:",inline"`
}
