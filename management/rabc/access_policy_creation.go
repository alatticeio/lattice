package rbac

//
//import "context"
//
//// 创建访问策略
//func createSamplePolicy(svc AccessPolicyService) error {
//	policy := &AccessPolicy{
//		Name:        "限制数据节点访问",
//		GroupId:     1,
//		Priority:    100,
//		Effect:      "allow",
//		Description: "允许带有data标签的节点之间互相访问",
//		Status:      true,
//	}
//
//	if err := svc.CreatePolicy(context.Background(), policy); err != nil {
//		return err
//	}
//
//	// 添加规则
//	rule := &AccessRule{
//		PolicyId:   policy.ID,
//		SourceType: "tag",
//		SourceId:   "data",
//		TargetType: "tag",
//		TargetId:   "data",
//		Actions:    []string{"connect", "transfer"},
//		Conditions: JSON{
//			"max_bandwidth": "100MB/s",
//			"time_window": {
//				"start": "09:00",
//				"end":   "18:00",
//			},
//		},
//	}
//
//	return svc.AddRule(context.Background(), rule)
//}
