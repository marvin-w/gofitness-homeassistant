package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeOptions(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "options.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaults(t *testing.T) {
	t.Setenv("GOFITNESS_OPTIONS", filepath.Join(t.TempDir(), "missing.json"))
	cfg := Load()
	if cfg.Port != 8099 || cfg.DataDir != "/data" || cfg.DefaultLang != "de" {
		t.Errorf("defaults = %+v", cfg)
	}
	if !cfg.PublishSensors {
		t.Error("sensor publishing should default to on")
	}
	if cfg.DBPath() != "/data/gofitness.db" {
		t.Errorf("db path = %q", cfg.DBPath())
	}
}

func TestOptionsFileIsApplied(t *testing.T) {
	path := writeOptions(t, `{
		"anthropic_api_key": "sk-ant-xyz",
		"ai_model": "claude-sonnet-5",
		"default_language": "en",
		"publish_sensors": false,
		"log_level": "debug"
	}`)
	t.Setenv("GOFITNESS_OPTIONS", path)

	cfg := Load()
	if cfg.AnthropicAPIKey != "sk-ant-xyz" || cfg.AIModel != "claude-sonnet-5" {
		t.Errorf("ai options not applied: %+v", cfg)
	}
	if cfg.DefaultLang != "en" {
		t.Errorf("language = %q", cfg.DefaultLang)
	}
	if cfg.PublishSensors {
		t.Error("an explicit false for publish_sensors must win over the default")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("log level = %q", cfg.LogLevel)
	}
}

// An omitted boolean must keep its default rather than silently becoming false.
func TestOmittedBoolKeepsDefault(t *testing.T) {
	t.Setenv("GOFITNESS_OPTIONS", writeOptions(t, `{"log_level":"warn"}`))
	if cfg := Load(); !cfg.PublishSensors {
		t.Error("publish_sensors flipped to false when omitted")
	}
}

func TestMalformedOptionsKeepDefaults(t *testing.T) {
	t.Setenv("GOFITNESS_OPTIONS", writeOptions(t, `{not json`))
	cfg := Load()
	if cfg.Port != 8099 || cfg.DataDir != "/data" || !cfg.PublishSensors {
		t.Errorf("a broken options file wiped the defaults: %+v", cfg)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	t.Setenv("GOFITNESS_OPTIONS", writeOptions(t, `{"default_language":"de"}`))
	t.Setenv("GOFITNESS_PORT", "9000")
	t.Setenv("GOFITNESS_DATA_DIR", "/tmp/gofitness")
	t.Setenv("GOFITNESS_LANG", "en")
	t.Setenv("GOFITNESS_LOG_LEVEL", "debug")

	cfg := Load()
	if cfg.Port != 9000 {
		t.Errorf("port = %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/gofitness" || cfg.DBPath() != "/tmp/gofitness/gofitness.db" {
		t.Errorf("data dir = %q, db = %q", cfg.DataDir, cfg.DBPath())
	}
	if cfg.DefaultLang != "en" {
		t.Errorf("language = %q, env should win", cfg.DefaultLang)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("log level = %q", cfg.LogLevel)
	}
}

// The add-on option is the documented place for the key; the environment
// variable is only a fallback for running outside Home Assistant.
func TestOptionsKeyBeatsEnvironment(t *testing.T) {
	t.Setenv("GOFITNESS_OPTIONS", writeOptions(t, `{"anthropic_api_key":"from-options"}`))
	t.Setenv("ANTHROPIC_API_KEY", "from-env")
	if cfg := Load(); cfg.AnthropicAPIKey != "from-options" {
		t.Errorf("key = %q, want the add-on option", cfg.AnthropicAPIKey)
	}
}

func TestEnvironmentKeyUsedWhenOptionEmpty(t *testing.T) {
	t.Setenv("GOFITNESS_OPTIONS", writeOptions(t, `{}`))
	t.Setenv("ANTHROPIC_API_KEY", "from-env")
	if cfg := Load(); cfg.AnthropicAPIKey != "from-env" {
		t.Errorf("key = %q, want the environment fallback", cfg.AnthropicAPIKey)
	}
}

func TestInvalidValuesFallBack(t *testing.T) {
	t.Setenv("GOFITNESS_OPTIONS", writeOptions(t, `{"port": 0, "data_dir": "", "ai_model": ""}`))
	cfg := Load()
	if cfg.Port != 8099 || cfg.DataDir != "/data" || cfg.AIModel == "" {
		t.Errorf("invalid values not repaired: %+v", cfg)
	}
}

func TestLanguageNormalisation(t *testing.T) {
	for _, in := range []string{"en", "EN", "en-GB", "english"} {
		t.Setenv("GOFITNESS_OPTIONS", writeOptions(t, `{"default_language":"`+in+`"}`))
		if cfg := Load(); cfg.DefaultLang != "en" {
			t.Errorf("%q normalised to %q", in, cfg.DefaultLang)
		}
	}
	for _, in := range []string{"de", "DE", "de-AT", "fr", "klingon"} {
		t.Setenv("GOFITNESS_OPTIONS", writeOptions(t, `{"default_language":"`+in+`"}`))
		if cfg := Load(); cfg.DefaultLang != "de" {
			t.Errorf("%q normalised to %q, want the German default", in, cfg.DefaultLang)
		}
	}
}
