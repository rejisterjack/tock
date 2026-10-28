package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookie = "tock_session"
	loginCookie   = "tock_csrf"
)

// Manager issues session cookies and checks the admin password and API token.
type Manager struct {
	Password string
	Secret   []byte
	APIToken string
	Secure   bool
	Lifetime time.Duration
	now      func() time.Time
}

// New builds a manager. Lifetime is how long a dashboard session lasts.
func New(password, secret, apiToken string, secure bool, lifetime time.Duration) *Manager {
	return &Manager{
		Password: password,
		Secret:   []byte(secret),
		APIToken: apiToken,
		Secure:   secure,
		Lifetime: lifetime,
		now:      time.Now,
	}
}

type payload struct {
	Exp  int64  `json:"exp"`
	CSRF string `json:"csrf"`
}

// PasswordOK reports whether pw matches the admin password.
func (m *Manager) PasswordOK(pw string) bool {
	return fixedEqual(pw, m.Password)
}

// TokenOK reports whether token matches the API token.
func (m *Manager) TokenOK(token string) bool {
	return fixedEqual(token, m.APIToken)
}

// NewCSRF returns a random token for a form.
func NewCSRF() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// StartSession writes the signed session cookie and returns its CSRF token.
func (m *Manager) StartSession(w http.ResponseWriter) (string, error) {
	csrf, err := NewCSRF()
	if err != nil {
		return "", err
	}
	exp := m.now().Add(m.Lifetime).Unix()
	value, err := m.seal(payload{Exp: exp, CSRF: csrf})
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(m.Lifetime.Seconds()),
	})
	return csrf, nil
}

// ClearSession removes the session cookie.
func (m *Manager) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// SessionCSRF returns the CSRF token when the request has a live session.
func (m *Manager) SessionCSRF(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", false
	}
	p, err := m.open(c.Value)
	if err != nil {
		return "", false
	}
	if p.Exp < m.now().Unix() || p.CSRF == "" {
		return "", false
	}
	return p.CSRF, true
}

// CSRFOK reports whether the form or header token matches the session.
func (m *Manager) CSRFOK(r *http.Request) bool {
	want, ok := m.SessionCSRF(r)
	if !ok {
		return false
	}
	got := r.FormValue("csrf")
	if got == "" {
		got = r.Header.Get("X-CSRF-Token")
	}
	return fixedEqual(got, want)
}

// SetLoginCSRF sets the cookie used to protect the login form and returns the token.
func (m *Manager) SetLoginCSRF(w http.ResponseWriter) (string, error) {
	csrf, err := NewCSRF()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     loginCookie,
		Value:    csrf,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   3600,
	})
	return csrf, nil
}

// LoginCSRFOK reports whether the login form token matches the cookie.
func (m *Manager) LoginCSRFOK(r *http.Request) bool {
	c, err := r.Cookie(loginCookie)
	if err != nil {
		return false
	}
	return fixedEqual(r.FormValue("csrf"), c.Value)
}

// Bearer extracts an Authorization bearer token.
func Bearer(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func (m *Manager) seal(p payload) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, m.Secret)
	mac.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (m *Manager) open(value string) (payload, error) {
	body, sig, ok := strings.Cut(value, ".")
	if !ok {
		return payload{}, errors.New("malformed session")
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return payload{}, err
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return payload{}, err
	}
	mac := hmac.New(sha256.New, m.Secret)
	mac.Write(raw)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return payload{}, errors.New("bad session signature")
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return payload{}, err
	}
	return p, nil
}

func fixedEqual(a, b string) bool {
	ah := sha256.Sum256([]byte(a))
	bh := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}
