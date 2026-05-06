package config

import (
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Auth     AuthConfig     `yaml:"auth"`
	Database DatabaseConfig `yaml:"database"`
	Settings SettingsConfig `yaml:"settings"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type AuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type SettingsConfig struct {
	MaxRequests            int      `yaml:"max_requests"`
	MaxRequestBodyBytes    int64    `yaml:"max_request_body_bytes"`
	MaxResponseBodyBytes   int64    `yaml:"max_response_body_bytes"`
	CleanupIntervalSeconds int      `yaml:"cleanup_interval_seconds"`
	SessionTTLHours        int      `yaml:"session_ttl_hours"`
	AllowedWSOrigins       []string `yaml:"allowed_ws_origins"`
	RedactionEnabled       bool     `yaml:"redaction_enabled"`
	RedactionHeaders       []string `yaml:"redaction_headers"`
	RedactionFields        []string `yaml:"redaction_fields"`
	AlertWebhookURL        string   `yaml:"alert_webhook_url"`
	AlertMinSentStatus     int      `yaml:"alert_min_sent_status"`
	AlertOnSentError       bool     `yaml:"alert_on_sent_error"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Auth: AuthConfig{
			Username: "admin",
			Password: "inspector123",
		},
		Database: DatabaseConfig{
			Path: "./inspector.db",
		},
		Settings: SettingsConfig{
			MaxRequests:            10000,
			MaxRequestBodyBytes:    1024 * 1024,
			MaxResponseBodyBytes:   2 * 1024 * 1024,
			CleanupIntervalSeconds: 30,
			SessionTTLHours:        12,
			RedactionEnabled:       true,
			RedactionHeaders: []string{
				"Authorization",
				"Cookie",
				"Set-Cookie",
				"X-Api-Key",
				"Api-Key",
				"Proxy-Authorization",
			},
			RedactionFields: []string{
				"password",
				"passwd",
				"secret",
				"token",
				"api_key",
				"apikey",
				"access_token",
				"refresh_token",
				"client_secret",
			},
			AlertWebhookURL:    "",
			AlertMinSentStatus: 500,
			AlertOnSentError:   true,
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	applyEnvOverrides(cfg)

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	applyStringEnv("INSPECTOR_SERVER_HOST", &cfg.Server.Host)
	applyIntEnv("INSPECTOR_SERVER_PORT", &cfg.Server.Port)

	applyStringEnv("INSPECTOR_AUTH_USERNAME", &cfg.Auth.Username)
	applyStringEnv("INSPECTOR_AUTH_PASSWORD", &cfg.Auth.Password)

	applyStringEnv("INSPECTOR_DATABASE_PATH", &cfg.Database.Path)

	applyIntEnv("INSPECTOR_SETTINGS_MAX_REQUESTS", &cfg.Settings.MaxRequests)
	applyInt64Env("INSPECTOR_SETTINGS_MAX_REQUEST_BODY_BYTES", &cfg.Settings.MaxRequestBodyBytes)
	applyInt64Env("INSPECTOR_SETTINGS_MAX_RESPONSE_BODY_BYTES", &cfg.Settings.MaxResponseBodyBytes)
	applyIntEnv("INSPECTOR_SETTINGS_CLEANUP_INTERVAL_SECONDS", &cfg.Settings.CleanupIntervalSeconds)
	applyIntEnv("INSPECTOR_SETTINGS_SESSION_TTL_HOURS", &cfg.Settings.SessionTTLHours)
	applyCSVEnv("INSPECTOR_SETTINGS_ALLOWED_WS_ORIGINS", &cfg.Settings.AllowedWSOrigins)

	applyBoolEnv("INSPECTOR_SETTINGS_REDACTION_ENABLED", &cfg.Settings.RedactionEnabled)
	applyCSVEnv("INSPECTOR_SETTINGS_REDACTION_HEADERS", &cfg.Settings.RedactionHeaders)
	applyCSVEnv("INSPECTOR_SETTINGS_REDACTION_FIELDS", &cfg.Settings.RedactionFields)

	applyStringEnv("INSPECTOR_SETTINGS_ALERT_WEBHOOK_URL", &cfg.Settings.AlertWebhookURL)
	applyIntEnv("INSPECTOR_SETTINGS_ALERT_MIN_SENT_STATUS", &cfg.Settings.AlertMinSentStatus)
	applyBoolEnv("INSPECTOR_SETTINGS_ALERT_ON_SENT_ERROR", &cfg.Settings.AlertOnSentError)
}

func applyStringEnv(envName string, target *string) {
	if target == nil {
		return
	}
	if value, ok := os.LookupEnv(envName); ok {
		*target = strings.TrimSpace(value)
	}
}

func applyIntEnv(envName string, target *int) {
	if target == nil {
		return
	}
	value, ok := os.LookupEnv(envName)
	if !ok {
		return
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		*target = parsed
	}
}

func applyInt64Env(envName string, target *int64) {
	if target == nil {
		return
	}
	value, ok := os.LookupEnv(envName)
	if !ok {
		return
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err == nil {
		*target = parsed
	}
}

func applyBoolEnv(envName string, target *bool) {
	if target == nil {
		return
	}
	value, ok := os.LookupEnv(envName)
	if !ok {
		return
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err == nil {
		*target = parsed
	}
}

func applyCSVEnv(envName string, target *[]string) {
	if target == nil {
		return
	}
	value, ok := os.LookupEnv(envName)
	if !ok {
		return
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	*target = out
}
