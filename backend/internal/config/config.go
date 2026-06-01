package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// Config holds runtime configuration loaded from environment.
type Config struct {
	Port          string
	DBPath        string
	AllowedOrigin string

	// LLM provider selection. One of:
	// "freellmapi" (recommended), "gemini", "groq", "openai", "openrouter",
	// "ollama", "openai-compatible".
	LLMProvider string
	LLMAPIKey   string
	LLMModel    string
	LLMBaseURL  string // only used for openai-compatible custom endpoints
}

// Load reads env vars (and a local .env file if present) and applies defaults.
func Load() Config {
	loadDotEnv(".env")

	cfg := Config{
		Port:          getenv("PORT", "8080"),
		DBPath:        getenv("DB_PATH", "./makesense.db"),
		AllowedOrigin: getenv("ALLOWED_ORIGIN", "http://localhost:3000"),
		LLMProvider:   strings.ToLower(getenv("LLM_PROVIDER", "freellmapi")),
		LLMBaseURL:    os.Getenv("LLM_BASE_URL"),
	}

	// Each provider has its own preferred env var names so you can keep several
	// keys side-by-side in the same .env and swap providers by changing one var.
	switch cfg.LLMProvider {
	case "gemini":
		cfg.LLMAPIKey = os.Getenv("GEMINI_API_KEY")
		cfg.LLMModel = getenv("GEMINI_MODEL", "gemini-2.0-flash")
	case "groq":
		cfg.LLMAPIKey = os.Getenv("GROQ_API_KEY")
		cfg.LLMModel = getenv("GROQ_MODEL", "llama-3.3-70b-versatile")
	case "openai":
		cfg.LLMAPIKey = os.Getenv("OPENAI_API_KEY")
		cfg.LLMModel = getenv("OPENAI_MODEL", "gpt-4o-mini")
	case "openrouter":
		cfg.LLMAPIKey = os.Getenv("OPENROUTER_API_KEY")
		cfg.LLMModel = getenv("OPENROUTER_MODEL", "meta-llama/llama-3.3-70b-instruct:free")
	case "ollama":
		cfg.LLMAPIKey = os.Getenv("OLLAMA_API_KEY")
		cfg.LLMModel = getenv("OLLAMA_MODEL", "llama3.1:8b")
	case "openai-compatible":
		cfg.LLMAPIKey = os.Getenv("LLM_API_KEY")
		cfg.LLMModel = os.Getenv("LLM_MODEL")
	case "freellmapi":
		cfg.LLMAPIKey = firstNonEmpty(os.Getenv("FREELLMAPI_API_KEY"), os.Getenv("LLM_API_KEY"))
		cfg.LLMModel = firstNonEmpty(os.Getenv("FREELLMAPI_MODEL"), os.Getenv("LLM_MODEL"), "auto")
		if cfg.LLMBaseURL == "" {
			cfg.LLMBaseURL = getenv("FREELLMAPI_BASE_URL", "http://localhost:3001/v1")
		}
	}

	if cfg.LLMAPIKey == "" && cfg.LLMProvider != "ollama" {
		log.Printf("WARNING: no API key for LLM provider %q — analysis calls will fail", cfg.LLMProvider)
	}
	return cfg
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv is a minimal .env loader (KEY=VALUE, one per line, # comments).
// We avoid adding a dependency just for this.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		v = strings.Trim(v, `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}
