package dto

type PlatformSettingsRequest struct {
	NatsURL string `json:"nats_url"`
}

type PlatformSettingsResponse struct {
	NatsURL string `json:"nats_url"`
}
