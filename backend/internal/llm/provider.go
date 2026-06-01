package llm

import (
	"fmt"
	"strings"
)

// ProviderConfig is the subset of app config the LLM factory needs.
// Kept separate from config.Config so the llm package doesn't depend on config.
type ProviderConfig struct {
	Provider string // "freellmapi" | "gemini" | "groq" | "openai" | "openrouter" | "ollama" | "openai-compatible"
	APIKey   string
	Model    string
	BaseURL  string // only used by openai-compatible providers
}

// NewGenerator returns a JSONGenerator for the configured provider.
// Defaults are picked to be useful on each provider's free tier.
func NewGenerator(cfg ProviderConfig) (JSONGenerator, error) {
	switch strings.ToLower(cfg.Provider) {
	case "", "gemini":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini: API key not set (set GEMINI_API_KEY)")
		}
		model := cfg.Model
		if model == "" {
			model = "gemini-2.0-flash"
		}
		return NewGeminiClient(cfg.APIKey, model), nil

	case "groq":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("groq: API key not set (set GROQ_API_KEY)")
		}
		model := cfg.Model
		if model == "" {
			model = "llama-3.3-70b-versatile"
		}
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.groq.com/openai/v1"
		}
		return NewOpenAICompatibleClient(cfg.APIKey, model, base), nil

	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openai: API key not set (set OPENAI_API_KEY)")
		}
		model := cfg.Model
		if model == "" {
			model = "gpt-4o-mini"
		}
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		return NewOpenAICompatibleClient(cfg.APIKey, model, base), nil

	case "openrouter":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openrouter: API key not set (set OPENROUTER_API_KEY)")
		}
		model := cfg.Model
		if model == "" {
			model = "meta-llama/llama-3.3-70b-instruct:free"
		}
		base := cfg.BaseURL
		if base == "" {
			base = "https://openrouter.ai/api/v1"
		}
		return NewOpenAICompatibleClient(cfg.APIKey, model, base), nil

	case "ollama":
		model := cfg.Model
		if model == "" {
			model = "llama3.1:8b"
		}
		base := cfg.BaseURL
		if base == "" {
			base = "http://localhost:11434/v1"
		}
		// Ollama doesn't require an API key; pass whatever is set (usually empty).
		return NewOpenAICompatibleClient(cfg.APIKey, model, base), nil

	case "openai-compatible":
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("openai-compatible: LLM_BASE_URL not set")
		}
		if cfg.Model == "" {
			return nil, fmt.Errorf("openai-compatible: LLM_MODEL not set")
		}
		return NewOpenAICompatibleClient(cfg.APIKey, cfg.Model, cfg.BaseURL), nil

	case "freellmapi":
		base := cfg.BaseURL
		if base == "" {
			base = "http://localhost:3001/v1"
		}
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("freellmapi: unified API key not set (set FREELLMAPI_API_KEY)")
		}
		model := cfg.Model
		if model == "" {
			model = "auto"
		}
		return NewOpenAICompatibleClient(cfg.APIKey, model, base), nil

	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER: %q", cfg.Provider)
	}
}
