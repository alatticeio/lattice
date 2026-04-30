package llm

import (
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/config"
)

// NewClient creates an LLMClient from the AI configuration.
// Returns an error if the provider is unknown or APIKey is empty.
func NewClient(cfg config.AIConfig) (Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("ai.api-key is not configured")
	}

	switch cfg.Provider {
	case "anthropic", "":
		return NewAnthropicClient(cfg.APIKey, cfg.Model, cfg.BaseURL), nil

	case "deepseek":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = defaultDeepSeekBaseURL
		}
		model := cfg.Model
		if model == "" {
			model = defaultDeepSeekModel
		}
		return NewOpenAICompatClient(baseURL, cfg.APIKey, model), nil

	case "openai":
		model := cfg.Model
		if model == "" {
			model = defaultOpenAIModel
		}
		return NewOpenAICompatClient("https://api.openai.com/v1", cfg.APIKey, model), nil

	default:
		// Custom OpenAI-compatible endpoint — requires base-url
		if cfg.BaseURL != "" {
			return NewOpenAICompatClient(cfg.BaseURL, cfg.APIKey, cfg.Model), nil
		}
		return nil, fmt.Errorf("unknown ai.provider %q; set base-url for custom OpenAI-compatible endpoints", cfg.Provider)
	}
}
