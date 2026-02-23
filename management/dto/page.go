package dto

// PageRequest 通用的分页请求参数
type PageRequest struct {
	Page      int    `form:"page" json:"page"`           // 页码
	PageSize  int    `form:"pageSize" json:"pageSize"`   // 每页条数
	Keyword   string `form:"search" json:"search"`       // 搜索关键词
	Namespace string `form:"namespace" json:"namespace"` // 命名空间/隔离字段
}

// PageResult 通用的分页返回容器（使用泛型 T）
type PageResult[T any] struct {
	Total    int64 `json:"total"`    // 总条数
	Page     int   `json:"page"`     // 当前页码
	PageSize int   `json:"pageSize"` // 每页条数
	List     []T   `json:"list"`     // 数据列表，改为 List 比 Data 更语义化
}
