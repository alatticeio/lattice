package service

//import (
//	"context"
//	"fmt"
//	"linkany/management/entity"
//	"linkany/pkg/log"
//)
//
//type CheckAccessService interface {
//	CheckNodeAccess(ctx context.Context, sourceNodeID, targetNodeID uint, action string) (bool, error)
//	GetNodeTags(ctx context.Context, nodeID uint) ([]string, error)
//}
//
//var _ CheckAccessService = (*CheckAccessServiceImpl)(nil)
//
//type CheckAccessServiceImpl struct {
//	logger *log.Logger
//	*DatabaseService
//}
//
//func (s *CheckAccessServiceImpl) CheckNodeAccess(ctx context.Context, sourceNodeID, targetNodeID uint, action string) (bool, error) {
//	//TODO implement me
//	panic("implement me")
//}
//
//func (s *CheckAccessServiceImpl) GetNodeTags(ctx context.Context, nodeID uint) ([]string, error) {
//	//TODO implement me
//	panic("implement me")
//}
//
//func NewCheckAccessService(db *DatabaseService) *CheckAccessServiceImpl {
//	return &CheckAccessServiceImpl{DatabaseService: db, logger: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "check_access_service"))}
//}
//
//// 检查访问权限
//func (s *CheckAccessServiceImpl) CheckAccess(ctx context.Context, sourceNodeID, targetNodeID uint, action string) (bool, error) {
//	// 1. 获取节点信息
//	sourceNode, targetNode := &entity.Node{}, &entity.Node{}
//	if err := s.First(sourceNode, sourceNodeID).Error; err != nil {
//		return false, err
//	}
//	if err := s.First(targetNode, targetNodeID).Error; err != nil {
//		return false, err
//	}
//
//	// 2. 如果不在同一分组，直接拒绝
//	if sourceNode.NetworkID != targetNode.NetworkID {
//		return false, nil
//	}
//
//	// 3. 获取分组内所有适用的策略
//	var policies []entity.AccessPolicy
//	if err := s.Where("group_id = ? AND status = ?", sourceNode.NetworkID, true).
//		Order("priority DESC").Find(&policies).Error; err != nil {
//		return false, err
//	}
//
//	// 4. 获取节点标签
//	sourceTags, err := s.GetNodeTags(ctx, sourceNodeID)
//	if err != nil {
//		return false, err
//	}
//	targetTags, err := s.GetNodeTags(ctx, targetNodeID)
//	if err != nil {
//		return false, err
//	}
//
//	// 5. 评估每个策略
//	for _, policy := range policies {
//		var rules []entity.AccessRule
//		if err := s.Where("policy_id = ?", policy.ID).Find(&rules).Error; err != nil {
//			continue
//		}
//
//		// 检查是否有匹配的规则
//		for _, rule := range rules {
//			if s.matchRule(rule, sourceNodeID, targetNodeID, sourceTags, targetTags, action) {
//				// 记录访问日志
//				s.logAccess(ctx, sourceNodeID, targetNodeID, action, policy.ID, policy.Effect == "allow")
//				return policy.Effect == "allow", nil
//			}
//		}
//	}
//
//	// 6. 默认拒绝
//	s.logAccess(ctx, sourceNodeID, targetNodeID, action, 0, false)
//	return false, nil
//}
//
//// 规则匹配逻辑
//func (s *CheckAccessServiceImpl) matchRule(rule entity.AccessRule, sourceNodeID, targetNodeID uint,
//	sourceTags, targetTags []string, action string) bool {
//
//	// 检查源匹配
//	sourceMatched := false
//	switch rule.SourceType {
//	case "node":
//		sourceMatched = rule.SourceId == fmt.Sprint(sourceNodeID)
//	case "tag":
//		sourceMatched = contains(sourceTags, rule.SourceId)
//	case "all":
//		sourceMatched = true
//	}
//
//	if !sourceMatched {
//		return false
//	}
//
//	// 检查目标匹配
//	targetMatched := false
//	switch rule.TargetType {
//	case "node":
//		targetMatched = rule.TargetId == fmt.Sprint(targetNodeID)
//	case "tag":
//		targetMatched = contains(targetTags, rule.TargetId)
//	case "all":
//		targetMatched = true
//	}
//
//	if !targetMatched {
//		return false
//	}
//
//	// 检查操作是否允许
//	return contains(rule.Actions, action)
//}
//
//func contains(tags []string, id string) bool {
//	for _, tag := range tags {
//		if tag == id {
//			return true
//		}
//	}
//	return false
//}
//
//// 访问日志记录
//func (s *CheckAccessServiceImpl) logAccess(ctx context.Context, sourceNodeID, targetNodeID uint,
//	action string, policyID uint, result bool) {
//
//	log := &entity.AccessLog{
//		SourceNodeID: sourceNodeID,
//		TargetNodeID: targetNodeID,
//		Action:       action,
//		Result:       result,
//		PolicyId:     policyID,
//	}
//
//	if err := s.Create(log).Error; err != nil {
//		s.logger.Errorf("Failed to create access log", err)
//	}
//}
