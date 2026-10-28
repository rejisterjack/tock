package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr          = ":8080"
	defaultRetentionDays = 90
	defaultTick          = 15 * time.Second
	minPasswordLen       = 8
	minSecretLen         = 16
	sessionLifetime      = 7 * 24 * time.Hour
)

// Config is process configuration loaded from the environment.
type Config struct {
	DatabaseURL       string
	BaseURL           string
	AdminPassword     string
	SessionSecret     string
	APIToken          string
	SMTPHost          string
	SMTPPort          int
	SMTPUser          string
	SMTPPassword      string
	SMTPFrom          string
	PingRetentionDays int
	Demo              bool
	TrustProxy        bool
	Addr              string
	TickInterval      time.Duration
	SessionLifetime   time.Duration
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return Parse(os.Getenv)
}

// Parse reads configuration using getenv so tests can supply values.
func Parse(getenv func(string) string) (Config, error) {
	var missing []string
	need := func(key string) string {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}
	cfg := Config{
		DatabaseURL:     need("DATABASE_URL"),
		BaseURL:         strings.TrimRight(need("TOCK_BASE_URL"), "/"),
		AdminPassword:   need("TOCK_ADMIN_PASSWORD"),
		SessionSecret:   need("TOCK_SESSION_SECRET"),
		APIToken:        need("TOCK_API_TOKEN"),
		SMTPHost:        strings.TrimSpace(getenv("TOCK_SMTP_HOST")),
		SMTPUser:        getenv("TOCK_SMTP_USER"),
		SMTPPassword:    getenv("TOCK_SMTP_PASSWORD"),
		SMTPFrom:        strings.TrimSpace(getenv("TOCK_SMTP_FROM")),
		Addr:            strings.TrimSpace(getenv("TOCK_ADDR")),
		TickInterval:    defaultTick,
		SessionLifetime: sessionLifetime,
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	if len(cfg.AdminPassword) < minPasswordLen {
		return Config{}, fmt.Errorf("TOCK_ADMIN_PASSWORD must be at least %d characters", minPasswordLen)
	}
	if len(cfg.SessionSecret) < minSecretLen {
		return Config{}, fmt.Errorf("TOCK_SESSION_SECRET must be at least %d characters", minSecretLen)
	}
	if len(cfg.APIToken) < minSecretLen {
		return Config{}, fmt.Errorf("TOCK_API_TOKEN must be at least %d characters", minSecretLen)
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("TOCK_BASE_URL must be an absolute http or https URL")
	}
	if cfg.Addr == "" {
		cfg.Addr = defaultAddr
	}
	if port := strings.TrimSpace(getenv("TOCK_SMTP_PORT")); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return Config{}, fmt.Errorf("TOCK_SMTP_PORT must be a port number")
		}
		cfg.SMTPPort = n
	} else if cfg.SMTPHost != "" {
		cfg.SMTPPort = 587
	}
	days := defaultRetentionDays
	if raw := strings.TrimSpace(getenv("TOCK_PING_RETENTION_DAYS")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 3650 {
			return Config{}, fmt.Errorf("TOCK_PING_RETENTION_DAYS must be between 1 and 3650")
		}
		days = n
	}
	cfg.PingRetentionDays = days
	demo, err := parseBool(getenv, "TOCK_DEMO")
	if err != nil {
		return Config{}, err
	}
	cfg.Demo = demo
	trust, err := parseBool(getenv, "TOCK_TRUST_PROXY")
	if err != nil {
		return Config{}, err
	}
	cfg.TrustProxy = trust
	return cfg, nil
}

func parseBool(getenv func(string) string, key string) (bool, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return false, nil
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes":
		return true, nil
	case "0", "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", key)
	}
}

// SecureCookies is true when the public URL is https.
func (c Config) SecureCookies() bool {
	return strings.HasPrefix(c.BaseURL, "https://")
}

// Retention is how long ping rows are kept.
func (c Config) Retention() time.Duration {
	return time.Duration(c.PingRetentionDays) * 24 * time.Hour
}
