package vo

type AppKeyVo struct {
	ModelVo
	AppKey string `json:"appKey"`
	Status string `json:"status"`
}

type UserSettingsVo struct {
}
