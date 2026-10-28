package config

import (
	"strings"
	"testing"
)

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":        "postgres://tock:tock@localhost:5432/tock?sslmode=disable",
		"TOCK_BASE_URL":       "http://localhost:8080/",
		"TOCK_ADMIN_PASSWORD": "change-me-now",
		"TOCK_SESSION_SECRET": "change-me-session-secret",
		"TOCK_API_TOKEN":      "change-me-api-token",
	}
}

func parse(env map[string]string) (Config, error) {
	return Parse(func(k string) string { return env[k] })
}

func TestParseDefaults(t *testing.T) {
	cfg, err := parse(validEnv())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://localhost:8080" {
		t.Fatalf("base url %q", cfg.BaseURL)
	}
	if cfg.Addr != ":8080" || cfg.PingRetentionDays != 90 || cfg.Demo || cfg.SMTPPort != 0 {
		t.Fatalf("%+v", cfg)
	}
}

func TestParseRequiresSecrets(t *testing.T) {
	env := validEnv()
	delete(env, "DATABASE_URL")
	_, err := parse(env)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatal(err)
	}
	env = validEnv()
	env["TOCK_ADMIN_PASSWORD"] = "short"
	if _, err := parse(env); err == nil {
		t.Fatal("expected short password to fail")
	}
	env = validEnv()
	env["TOCK_SESSION_SECRET"] = "too-short"
	if _, err := parse(env); err == nil {
		t.Fatal("expected short session secret to fail")
	}
	env = validEnv()
	env["TOCK_BASE_URL"] = "ftp://example.com"
	if _, err := parse(env); err == nil {
		t.Fatal("expected bad base url to fail")
	}
	env = validEnv()
	env["TOCK_DEMO"] = "maybe"
	if _, err := parse(env); err == nil {
		t.Fatal("expected bad demo flag to fail")
	}
	env = validEnv()
	env["TOCK_DEMO"] = "true"
	env["TOCK_TRUST_PROXY"] = "true"
	env["TOCK_PING_RETENTION_DAYS"] = "14"
	env["TOCK_SMTP_HOST"] = "mailpit"
	cfg, err := parse(env)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Demo || !cfg.TrustProxy || cfg.PingRetentionDays != 14 || cfg.SMTPPort != 587 {
		t.Fatalf("%+v", cfg)
	}
	env["TOCK_TRUST_PROXY"] = "maybe"
	if _, err := parse(env); err == nil {
		t.Fatal("expected bad trust proxy flag to fail")
	}
}
