package vo

type PermissionVo struct {
	ID          uint64 `json:"id,string"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description"`
}
