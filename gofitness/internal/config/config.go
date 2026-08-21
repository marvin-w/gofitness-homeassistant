// Package config reads the add-on configuration. Home Assistant writes the
// user's options to /data/options.json; environment variables override it so
// the app can also be run outside the add-on for development.
package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// Config is the effective runtime configuration.
type Config struct {
	// Port the HTTP server listens on. Ingress talks to this port too.
	Port int `json:"port"`
	// DataDir holds the SQLite database. /data is persisted by Home Assistant.
	DataDir string `json:"data_dir"`
	// AnthropicAPIKey enables AI calorie estimation, including from photos.
	// Leave empty to use only the built-in local food table.
	AnthropicAPIKey string `json:"anthropic_api_key"`
	// AIModel overrides the Claude model used for estimates.
	AIModel string `json:"ai_model"`
	// DefaultLang is the language new profiles start in ("de" or "en").
	DefaultLang string `json:"default_language"`
	// PublishSensors mirrors weight, calories and streak into Home Assistant.
	PublishSensors bool `json:"publish_sensors"`
	// LogLevel is "debug", "info" or "warn".
	LogLevel string `json:"log_level"`
}

// Default returns the configuration used when nothing is set.
func Default() Config {
	return Config{
		Port:           8099,
		DataDir:        "/data",
		AIModel:        "claude-opus-5",
		DefaultLang:    "de",
		PublishSensors: true,
		LogLevel:       "info",
	}
}

// optionsFile is the add-on options written by the Supervisor.
const optionsFile = "/data/options.json"

// Load reads the add-on options, then applies environment overrides.
func Load() Config {
	cfg := Default()

	path := os.Getenv("GOFITNESS_OPTIONS")
	if path == "" {
		path = optionsFile
	}
	if raw, err := os.ReadFile(path); err == nil {
		// Options are decoded into a separate struct so a malformed file cannot
		// wipe the defaults — an add-on that starts with sane settings is
		// better than one that refuses to boot.
		var opts options
		if err := json.Unmarshal(raw, &opts); err == nil {
			merge(&cfg, opts)
		}
	}

	if v := os.Getenv("GOFITNESS_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Port = n
		}
	}
	if v := os.Getenv("GOFITNESS_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" && cfg.AnthropicAPIKey == "" {
		cfg.AnthropicAPIKey = v
	}
	if v := os.Getenv("GOFITNESS_AI_MODEL"); v != "" {
		cfg.AIModel = v
	}
	if v := os.Getenv("GOFITNESS_LANG"); v != "" {
		cfg.DefaultLang = v
	}
	if v := os.Getenv("GOFITNESS_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	cfg.DefaultLang = normalizeLang(cfg.DefaultLang)
	cfg.AnthropicAPIKey = strings.TrimSpace(cfg.AnthropicAPIKey)
	if cfg.Port <= 0 {
		cfg.Port = 8099
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}
	if cfg.AIModel == "" {
		cfg.AIModel = "claude-opus-5"
	}
	return cfg
}

// options mirrors Config for decoding. PublishSensors is a pointer so an
// omitted key keeps the default of true, while an explicit false wins.
type options struct {
	Port            int    `json:"port"`
	DataDir         string `json:"data_dir"`
	AnthropicAPIKey string `json:"anthropic_api_key"`
	AIModel         string `json:"ai_model"`
	DefaultLang     string `json:"default_language"`
	PublishSensors  *bool  `json:"publish_sensors"`
	LogLevel        string `json:"log_level"`
}

// merge copies fields that were actually set from src over dst.
func merge(dst *Config, src options) {
	if src.Port != 0 {
		dst.Port = src.Port
	}
	if src.DataDir != "" {
		dst.DataDir = src.DataDir
	}
	if src.AnthropicAPIKey != "" {
		dst.AnthropicAPIKey = src.AnthropicAPIKey
	}
	if src.AIModel != "" {
		dst.AIModel = src.AIModel
	}
	if src.DefaultLang != "" {
		dst.DefaultLang = src.DefaultLang
	}
	if src.LogLevel != "" {
		dst.LogLevel = src.LogLevel
	}
	if src.PublishSensors != nil {
		dst.PublishSensors = *src.PublishSensors
	}
}

func normalizeLang(s string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "en") {
		return "en"
	}
	return "de"
}

// DBPath returns the full path of the SQLite database.
func (c Config) DBPath() string {
	return strings.TrimRight(c.DataDir, "/") + "/gofitness.db"
}
