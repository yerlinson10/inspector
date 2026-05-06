package config

import (
	"os"

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

	return cfg, nil
}
