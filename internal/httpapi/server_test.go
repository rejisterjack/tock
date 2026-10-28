package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rejisterjack/tock/internal/app"
	"github.com/rejisterjack/tock/internal/config"
	"github.com/rejisterjack/tock/internal/model"
	"github.com/rejisterjack/tock/internal/notify"
	"github.com/rejisterjack/tock/internal/store/mem"
)

type nopSender struct{}

func (nopSender) Send(context.Context, model.Channel, notify.Message) error { return nil }

func newTestApp(t *testing.T) (*httptest.Server, *app.Service, *mem.Mem) {
	t.Helper()
	repo := mem.New()
	clock := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	svc := &app.Service{
		Repo: repo, Notifier: nopSender{}, BaseURL: "http://example.test",
		Retention: 24 * time.Hour,
		Now:       func() time.Time { return clock },
	}
	cfg := config.Config{
		BaseURL: "http://example.test", AdminPassword: "change-me-now",
		SessionSecret: "change-me-session-secret", APIToken: "change-me-api-token",
		SessionLifetime: time.Hour,
	}
	api, err := New(cfg, svc, nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	api.now = func() time.Time { return clock }
	ts := httptest.NewServer(api.Handler())
	t.Cleanup(ts.Close)
	return ts, svc, repo
}

func TestHealthAndAuthRejection(t *testing.T) {
	ts, _, _ := newTestApp(t)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health %d", res.StatusCode)
	}
	res, err = client.Get(ts.URL + "/api/v1/checks")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("api %d", res.StatusCode)
	}
	res, err = client.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("dashboard %d", res.StatusCode)
	}
}

func TestAPICreateThenPing(t *testing.T) {
	ts, svc, _ := newTestApp(t)
	body := []byte(`{"name":"nightly-backup","period_seconds":60,"grace_seconds":0}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/checks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer change-me-api-token")
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d %s", res.StatusCode, raw)
	}
	var created checkJSON
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(created.Curl, created.PingURL) || !strings.HasPrefix(created.PingURL, "http://example.test/ping/") {
		t.Fatalf("%+v", created)
	}
	token := strings.TrimPrefix(created.PingURL, "http://example.test/ping/")
	ping, err := http.Get(ts.URL + "/ping/" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer ping.Body.Close()
	text, _ := io.ReadAll(ping.Body)
	if ping.StatusCode != http.StatusOK || string(text) != "OK\n" {
		t.Fatalf("ping %d %q", ping.StatusCode, text)
	}
	missing, err := http.Get(ts.URL + "/ping/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing %d", missing.StatusCode)
	}

	start := svc.Now()
	svc.Now = func() time.Time { return start.Add(2 * time.Minute) }
	if err := svc.Tick(req.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pause(req.Context(), created.ID); err != nil {
		t.Fatal(err)
	}
	again, err := http.Post(ts.URL+"/ping/"+token, "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Body.Close()
	if again.StatusCode != http.StatusOK {
		t.Fatalf("paused ping %d", again.StatusCode)
	}
	got, err := svc.GetCheck(req.Context(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.StatusDown || got.LastPingAt == nil || !got.Paused {
		t.Fatalf("%+v", got)
	}
	pings, err := svc.ListPings(req.Context(), created.ID, 10)
	if err != nil || len(pings) != 2 {
		t.Fatalf("pings %d %v", len(pings), err)
	}
}

func TestCSRFAndLogin(t *testing.T) {
	ts, _, _ := newTestApp(t)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Get(ts.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	token := csrfValue(t, string(body))
	res, err = client.PostForm(ts.URL+"/login", map[string][]string{
		"csrf": {token}, "password": {"change-me-now"},
	})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("login %d", res.StatusCode)
	}
	res, err = client.Get(ts.URL + "/checks/new")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	formToken := csrfValue(t, string(body))
	res, err = client.PostForm(ts.URL+"/checks", map[string][]string{
		"name": {"backup"}, "period_value": {"1"}, "period_unit": {"hours"},
		"grace_value": {"5"}, "grace_unit": {"minutes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing csrf %d", res.StatusCode)
	}
	res, err = client.PostForm(ts.URL+"/checks", map[string][]string{
		"csrf": {formToken}, "name": {"backup"}, "period_value": {"1"}, "period_unit": {"hours"},
		"grace_value": {"5"}, "grace_unit": {"minutes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther || !strings.Contains(res.Header.Get("Location"), "/checks/") {
		t.Fatalf("create %d %s", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestRemoteIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ping/x", nil)
	req.RemoteAddr = "10.0.0.1:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.2")
	if got := remoteIP(req, false); got != "10.0.0.1" {
		t.Fatalf("direct %s", got)
	}
	if got := remoteIP(req, true); got != "203.0.113.5" {
		t.Fatalf("proxy %s", got)
	}
}

func csrfValue(t *testing.T, body string) string {
	t.Helper()
	const needle = `name="csrf" value="`
	i := strings.Index(body, needle)
	if i < 0 {
		t.Fatalf("csrf not in %s", body)
	}
	rest := body[i+len(needle):]
	j := strings.Index(rest, `"`)
	return rest[:j]
}
