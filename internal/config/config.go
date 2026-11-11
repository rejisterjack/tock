package config

import (
	"fmt"
	"net"
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
		// Prefer platform PORT (Render, Railway, etc.) when TOCK_ADDR is unset.
		if port := strings.TrimSpace(getenv("PORT")); port != "" {
			cfg.Addr = ":" + port
		} else {
			cfg.Addr = defaultAddr
		}
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
	if err := rejectExampleSecrets(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// rejectExampleSecrets refuses the passwords shipped in .env.example once the
// public URL is not a loopback address. Local Compose can still boot from the
// example file; a deployed host cannot.
func rejectExampleSecrets(cfg Config) error {
	if loopbackBase(cfg.BaseURL) {
		return nil
	}
	var bad []string
	if cfg.AdminPassword == "change-me-now" {
		bad = append(bad, "TOCK_ADMIN_PASSWORD")
	}
	if cfg.SessionSecret == "change-me-session-secret" {
		bad = append(bad, "TOCK_SESSION_SECRET")
	}
	if cfg.APIToken == "change-me-api-token" {
		bad = append(bad, "TOCK_API_TOKEN")
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf("replace the example values in %s before using a public TOCK_BASE_URL", strings.Join(bad, ", "))
}

func loopbackBase(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
