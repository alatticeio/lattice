package service

//
//import "linkany/management/entity"
//
//// PlanService is an interface for plan mapper
//type PlanService interface {
//	// List returns a list of plans
//	List() ([]*entity.Plan, error)
//	Get() (*entity.Plan, error)
//	Page() (*entity.Plan, error)
//}
//
//var (
//	_ PlanService = (*planServiceImpl)(nil)
//)
//
//type planServiceImpl struct {
//	*DatabaseService
//}
//
//func NewPlanService(db *DatabaseService) *planServiceImpl {
//	return &planServiceImpl{DatabaseService: db}
//}
//
//func (p planServiceImpl) List() ([]*entity.Plan, error) {
//	var plans []*entity.Plan
//	if err := p.Where("1=1").Find(&plans).Error; err != nil {
//		return nil, err
//	}
//
//	return plans, nil
//}
//
//func (p planServiceImpl) Get() (*entity.Plan, error) {
//	//TODO implement me
//	panic("implement me")
//}
//
//func (p planServiceImpl) Page() (*entity.Plan, error) {
//	//TODO implement me
//	panic("implement me")
//}
