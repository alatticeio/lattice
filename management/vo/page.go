package vo

// PageModel is a data transfer object for pagination, add 'form' is used to bind data from request
// like c.ShouldBindQuery using.
type PageModel struct {
	Total   int64 `json:"total" form:"total"`
	Page    int   `json:"page" form:"page"`
	Current int   `json:"current" form:"current""`
	Size    int   `json:"size" form:"size"`
}

type PageVo struct {
	Data interface{} `json:"data"`
	PageModel
}
