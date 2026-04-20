package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// Config holds runtime configuration loaded from environment.
type Config struct {
	Port           string
	DBPath         string
	GeminiAPIKey   string
	GeminiModel    string
	AllowedOrigin  string
}

// Load reads env vars (and a local .env file if present) and applies defaults.
func Load() Config {
	loadDotEnv(".env")

	cfg := Config{
		Port:          getenv("PORT", "8080"),
		DBPath:        getenv("DB_PATH", "./makesense.db"),
		GeminiAPIKey:  os.Getenv("GEMINI_API_KEY"),
		GeminiModel:   getenv("GEMINI_MODEL", "gemini-2.0-flash"),
		AllowedOrigin: getenv("ALLOWED_ORIGIN", "http://localhost:3000"),
	}

	if cfg.GeminiAPIKey == "" {
		log.Println("WARNING: GEMINI_API_KEY is not set — analysis calls will fail")
	}
	return cfg
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
