package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	m := New("change-me-now", "change-me-session-secret", "change-me-api-token", false, time.Hour)
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	rec := httptest.NewRecorder()
	csrf, err := m.StartSession(rec)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", rec.Header().Get("Set-Cookie"))
	got, ok := m.SessionCSRF(req)
	if !ok || got != csrf {
		t.Fatalf("csrf %q ok %v", got, ok)
	}
	m.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, ok := m.SessionCSRF(req); ok {
		t.Fatal("expired session should fail")
	}
}

func TestPasswordAndBearer(t *testing.T) {
	m := New("change-me-now", "change-me-session-secret", "change-me-api-token", false, time.Hour)
	if !m.PasswordOK("change-me-now") || m.PasswordOK("change-me-not") {
		t.Fatal("password compare")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer change-me-api-token")
	tok, ok := Bearer(req)
	if !ok || !m.TokenOK(tok) {
		t.Fatal("bearer")
	}
	req.Header.Set("Authorization", "Basic nope")
	if _, ok := Bearer(req); ok {
		t.Fatal("basic should not parse as bearer")
	}
}

func TestTamperedSession(t *testing.T) {
	m := New("change-me-now", "change-me-session-secret", "change-me-api-token", false, time.Hour)
	rec := httptest.NewRecorder()
	if _, err := m.StartSession(rec); err != nil {
		t.Fatal(err)
	}
	cookie := rec.Result().Cookies()[0].Value
	cookie = "A" + cookie[1:]
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "tock_session", Value: cookie})
	if _, ok := m.SessionCSRF(req); ok {
		t.Fatal("tampered cookie should fail")
	}
}
