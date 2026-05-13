package dto

type PlatformSettingsRequest struct {
	NatsURL string `json:"nats_url" binding:"required"`
}

type PlatformSettingsResponse struct {
	NatsURL string `json:"nats_url"`
}
