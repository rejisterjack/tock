package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rejisterjack/tock/internal/app"
	"github.com/rejisterjack/tock/internal/config"
	"github.com/rejisterjack/tock/internal/model"
	"github.com/rejisterjack/tock/internal/notify"
	"github.com/rejisterjack/tock/internal/store/mem"
)

func noRedirectClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func doForm(t *testing.T, client *http.Client, rawURL string, fields map[string]string) (int, http.Header, string) {
	t.Helper()
	vals := url.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	res, err := client.PostForm(rawURL, vals)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, string(body)
}

func doJSON(t *testing.T, method, rawURL, token string, payload []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}

func TestDashboardMutations(t *testing.T) {
	ts, _, _ := newTestApp(t)
	client := noRedirectClient(t)
	token := loginToken(t, client, ts.URL)
	code, hdr, _ := doForm(t, client, ts.URL+"/login", map[string]string{
		"csrf": token, "password": "change-me-now",
	})
	if code != http.StatusSeeOther || hdr.Get("Location") != "/" {
		t.Fatalf("login %d %s", code, hdr.Get("Location"))
	}

	res, err := client.Get(ts.URL + "/checks/new")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(res.Body)
	res.Body.Close()
	csrf := csrfValue(t, string(page))
	code, hdr, _ = doForm(t, client, ts.URL+"/checks", map[string]string{
		"csrf": csrf, "name": "dashboard-backup",
		"period_value": "1", "period_unit": "hours",
		"grace_value": "5", "grace_unit": "minutes",
	})
	loc := hdr.Get("Location")
	if code != http.StatusSeeOther || !strings.HasPrefix(loc, "/checks/") {
		t.Fatalf("create %d %s", code, loc)
	}

	res, err = client.Get(ts.URL + loc + "/edit")
	if err != nil {
		t.Fatal(err)
	}
	page, _ = io.ReadAll(res.Body)
	res.Body.Close()
	csrf = csrfValue(t, string(page))
	code, hdr, _ = doForm(t, client, ts.URL+loc, map[string]string{
		"csrf": csrf, "name": "dashboard-renamed",
		"period_value": "2", "period_unit": "hours",
		"grace_value": "10", "grace_unit": "minutes",
	})
	if code != http.StatusSeeOther || hdr.Get("Location") != loc {
		t.Fatalf("edit %d %s", code, hdr.Get("Location"))
	}

	res, err = client.Get(ts.URL + loc)
	if err != nil {
		t.Fatal(err)
	}
	page, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(page), "dashboard-renamed") || !strings.Contains(string(page), "http://example.test/ping/") {
		t.Fatalf("detail missing name or ping url")
	}
	csrf = csrfValue(t, string(page))
	code, _, _ = doForm(t, client, ts.URL+loc+"/pause", map[string]string{"csrf": csrf})
	if code != http.StatusSeeOther {
		t.Fatalf("pause %d", code)
	}
	res, err = client.Get(ts.URL + loc)
	if err != nil {
		t.Fatal(err)
	}
	page, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(page), "Resume") || !strings.Contains(strings.ToLower(string(page)), "paused") {
		t.Fatal("expected paused detail")
	}
	csrf = csrfValue(t, string(page))
	code, _, _ = doForm(t, client, ts.URL+loc+"/resume", map[string]string{"csrf": csrf})
	if code != http.StatusSeeOther {
		t.Fatalf("resume %d", code)
	}

	res, err = client.Get(ts.URL + "/channels")
	if err != nil {
		t.Fatal(err)
	}
	page, _ = io.ReadAll(res.Body)
	res.Body.Close()
	csrf = csrfValue(t, string(page))
	code, hdr, body := doForm(t, client, ts.URL+"/channels", map[string]string{
		"csrf": csrf, "name": "ops-mail", "kind": "email", "enabled": "true", "to": "ops@example.com",
	})
	if code != http.StatusSeeOther || hdr.Get("Location") != "/channels" {
		t.Fatalf("channel %d %s %s", code, hdr.Get("Location"), body)
	}
	res, err = client.Get(ts.URL + "/channels")
	if err != nil {
		t.Fatal(err)
	}
	page, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(page), "ops-mail") || !strings.Contains(string(page), "ops@example.com") {
		t.Fatal("channel was not saved")
	}

	res, err = client.Get(ts.URL + loc)
	if err != nil {
		t.Fatal(err)
	}
	page, _ = io.ReadAll(res.Body)
	res.Body.Close()
	csrf = csrfValue(t, string(page))
	code, hdr, _ = doForm(t, client, ts.URL+loc+"/delete", map[string]string{"csrf": csrf})
	if code != http.StatusSeeOther || hdr.Get("Location") != "/" {
		t.Fatalf("delete %d %s", code, hdr.Get("Location"))
	}
	res, err = client.Get(ts.URL + loc)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted check %d", res.StatusCode)
	}

	res, err = client.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	page, _ = io.ReadAll(res.Body)
	res.Body.Close()
	csrf = csrfValue(t, string(page))
	code, hdr, _ = doForm(t, client, ts.URL+"/logout", map[string]string{"csrf": csrf})
	if code != http.StatusSeeOther || hdr.Get("Location") != "/login" {
		t.Fatalf("logout %d %s", code, hdr.Get("Location"))
	}
	res, err = client.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/login" {
		t.Fatalf("after logout %d %s", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestAPILifecycle(t *testing.T) {
	ts, svc, _ := newTestApp(t)
	const token = "change-me-api-token"
	code, body := doJSON(t, http.MethodGet, ts.URL+"/api/v1/checks", "not-the-token", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("wrong token %d %s", code, body)
	}
	code, body = doJSON(t, http.MethodPost, ts.URL+"/api/v1/checks", token, []byte(`{"name":"backup","period_seconds":3600,"grace_seconds":60,"extra":true}`))
	if code != http.StatusBadRequest {
		t.Fatalf("unknown field %d %s", code, body)
	}
	huge := bytes.Repeat([]byte("a"), 1<<20+32)
	code, _ = doJSON(t, http.MethodPost, ts.URL+"/api/v1/checks", token, huge)
	if code != http.StatusBadRequest {
		t.Fatalf("oversize body %d", code)
	}

	code, body = doJSON(t, http.MethodPost, ts.URL+"/api/v1/checks", token, []byte(`{"name":"backup","period_seconds":3600,"grace_seconds":60}`))
	if code != http.StatusCreated {
		t.Fatalf("create %d %s", code, body)
	}
	var created checkJSON
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	code, body = doJSON(t, http.MethodPatch, ts.URL+"/api/v1/checks/"+created.ID, token, []byte(`{"name":"renamed"}`))
	if code != http.StatusOK {
		t.Fatalf("patch %d %s", code, body)
	}
	var patched checkJSON
	if err := json.Unmarshal(body, &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Name != "renamed" || patched.PeriodSeconds != 3600 || patched.GraceSeconds != 60 {
		t.Fatalf("partial patch %+v", patched)
	}

	pingURL := ts.URL + "/ping/" + strings.TrimPrefix(created.PingURL, "http://example.test/ping/")
	res, err := http.Get(pingURL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	code, body = doJSON(t, http.MethodGet, ts.URL+"/api/v1/checks", token, nil)
	var listed struct {
		Checks []checkJSON `json:"checks"`
	}
	if err := json.Unmarshal(body, &listed); err != nil || code != http.StatusOK || len(listed.Checks) != 1 || listed.Checks[0].Name != "renamed" {
		t.Fatalf("list %d %s %v", code, body, err)
	}
	code, body = doJSON(t, http.MethodGet, ts.URL+"/api/v1/checks/"+created.ID+"/pings", token, nil)
	var pings struct {
		Pings []model.Ping `json:"pings"`
	}
	if err := json.Unmarshal(body, &pings); err != nil || code != http.StatusOK || len(pings.Pings) != 1 {
		t.Fatalf("pings %d %s %v", code, body, err)
	}

	on := true
	channel, _ := json.Marshal(map[string]any{
		"kind": "email", "name": "ops", "enabled": on,
		"config": map[string]any{"to": []string{"ops@example.com"}},
	})
	code, body = doJSON(t, http.MethodPost, ts.URL+"/api/v1/channels", token, channel)
	if code != http.StatusCreated {
		t.Fatalf("channel %d %s", code, body)
	}
	off, _ := json.Marshal(map[string]any{
		"kind": "email", "name": "quiet", "enabled": false,
		"config": map[string]any{"to": []string{"quiet@example.com"}},
	})
	code, body = doJSON(t, http.MethodPost, ts.URL+"/api/v1/channels", token, off)
	if code != http.StatusCreated {
		t.Fatalf("disabled channel %d %s", code, body)
	}

	start := svc.Now()
	svc.Now = func() time.Time { return start.Add(2 * time.Hour) }
	if err := svc.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	code, body = doJSON(t, http.MethodGet, ts.URL+"/api/v1/checks/"+created.ID+"/alerts", token, nil)
	var alerts struct {
		Alerts []model.Alert `json:"alerts"`
	}
	if err := json.Unmarshal(body, &alerts); err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK || len(alerts.Alerts) != 1 || alerts.Alerts[0].ChannelName != "ops" || alerts.Alerts[0].Status != model.AlertSent {
		t.Fatalf("alerts %d %+v", code, alerts.Alerts)
	}

	code, body = doJSON(t, http.MethodPost, ts.URL+"/api/v1/checks/"+created.ID+"/pause", token, []byte(`{}`))
	if code != http.StatusOK || !strings.Contains(string(body), `"paused":true`) {
		t.Fatalf("pause %d %s", code, body)
	}
	code, body = doJSON(t, http.MethodPost, ts.URL+"/api/v1/checks/"+created.ID+"/resume", token, []byte(`{}`))
	if code != http.StatusOK || !strings.Contains(string(body), `"paused":false`) {
		t.Fatalf("resume %d %s", code, body)
	}
	code, _ = doJSON(t, http.MethodDelete, ts.URL+"/api/v1/checks/"+created.ID, token, nil)
	if code != http.StatusNoContent {
		t.Fatalf("delete %d", code)
	}
	code, _ = doJSON(t, http.MethodGet, ts.URL+"/api/v1/checks/"+created.ID, token, nil)
	if code != http.StatusNotFound {
		t.Fatalf("after delete %d", code)
	}
}

func TestDemoHTTPIsReadOnly(t *testing.T) {
	repo := mem.New()
	clock := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	svc := &app.Service{
		Repo: repo, Notifier: notify.Guard{Demo: true, Next: nopSender{}},
		BaseURL: "http://example.test", Demo: true,
		Now: func() time.Time { return clock },
	}
	if err := repo.CreateCheck(t.Context(), model.Check{
		ID: "demo-1", Name: "nightly-backup", PingToken: "demo-token",
		PeriodSeconds: 3600, GraceSeconds: 60, Status: model.StatusUp,
		CreatedAt: clock, UpdatedAt: clock,
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		BaseURL: "http://example.test", AdminPassword: "change-me-now",
		SessionSecret: "change-me-session-secret", APIToken: "change-me-api-token",
		SessionLifetime: time.Hour, Demo: true,
	}
	api, err := New(cfg, svc, nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(api.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(string(page), "nightly-backup") {
		t.Fatalf("demo page %d", res.StatusCode)
	}
	res, err = http.Post(ts.URL+"/checks", "application/x-www-form-urlencoded", strings.NewReader("name=nope"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("demo create %d", res.StatusCode)
	}
	res, err = http.Get(ts.URL + "/ping/demo-token")
	if err != nil {
		t.Fatal(err)
	}
	text, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(text) != "OK\n" {
		t.Fatalf("demo ping %d %q", res.StatusCode, text)
	}
	got, err := repo.GetCheck(t.Context(), "demo-1")
	if err != nil || got.LastPingAt != nil || got.Status != model.StatusUp {
		t.Fatalf("%+v %v", got, err)
	}
	code, body := doJSON(t, http.MethodPost, ts.URL+"/api/v1/checks", "change-me-api-token", []byte(`{"name":"x","period_seconds":60,"grace_seconds":0}`))
	if code != http.StatusForbidden || !strings.Contains(string(body), "read-only") {
		t.Fatalf("demo api %d %s", code, body)
	}
}

func TestLoginSuccessClearsLockout(t *testing.T) {
	ts, _, _ := newTestApp(t)
	client := noRedirectClient(t)
	token := loginToken(t, client, ts.URL)
	for i := 0; i < 7; i++ {
		code, _, _ := doForm(t, client, ts.URL+"/login", map[string]string{
			"csrf": token, "password": "wrong-password",
		})
		if code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status %d", i+1, code)
		}
	}
	code, hdr, _ := doForm(t, client, ts.URL+"/login", map[string]string{
		"csrf": token, "password": "change-me-now",
	})
	if code != http.StatusSeeOther || hdr.Get("Location") != "/" {
		t.Fatalf("success after failures %d %s", code, hdr.Get("Location"))
	}
	for i := 0; i < 8; i++ {
		code, _, _ = doForm(t, client, ts.URL+"/login", map[string]string{
			"csrf": token, "password": "wrong-password",
		})
		if code != http.StatusUnauthorized {
			t.Fatalf("post-reset attempt %d status %d", i+1, code)
		}
	}
	code, _, body := doForm(t, client, ts.URL+"/login", map[string]string{
		"csrf": token, "password": "change-me-now",
	})
	if code != http.StatusTooManyRequests || !strings.Contains(body, "Too many attempts") {
		t.Fatalf("lockout after reset %d %s", code, body)
	}
}

func TestPingPathIsRedacted(t *testing.T) {
	repo := mem.New()
	svc := &app.Service{Repo: repo, Notifier: nopSender{}, BaseURL: "http://example.test"}
	cfg := config.Config{
		BaseURL: "http://example.test", AdminPassword: "change-me-now",
		SessionSecret: "change-me-session-secret", APIToken: "change-me-api-token",
		SessionLifetime: time.Hour,
	}
	var buf bytes.Buffer
	api, err := New(cfg, svc, slog.New(slog.NewJSONHandler(&buf, nil)), "test")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(api.Handler())
	t.Cleanup(ts.Close)
	res, err := http.Get(ts.URL + "/ping/super-secret-token")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	logged := buf.String()
	if strings.Contains(logged, "super-secret-token") || !strings.Contains(logged, "/ping/:token") {
		t.Fatalf("log %s", logged)
	}
}
