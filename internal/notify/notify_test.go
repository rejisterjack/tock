package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rejisterjack/tock/internal/model"
)

func sampleMessage() Message {
	last := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	return Message{
		Event: EventDown,
		Check: model.Check{
			ID:            "chk",
			Name:          "nightly-backup",
			Status:        model.StatusDown,
			PeriodSeconds: 86400,
			GraceSeconds:  3600,
			LastPingAt:    &last,
		},
		SentAt:       time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC),
		DashboardURL: "http://localhost:8080/checks/chk",
	}
}

func TestTextOmitsToken(t *testing.T) {
	msg := sampleMessage()
	msg.Check.PingToken = "secret-token"
	body := Text(msg)
	if strings.Contains(body, "secret-token") {
		t.Fatal(body)
	}
	if !strings.Contains(body, "nightly-backup") || !strings.Contains(body, "Last ping:") || !strings.Contains(body, "1d") {
		t.Fatal(body)
	}
	if Subject(msg) != "Tock: nightly-backup is down" {
		t.Fatal(Subject(msg))
	}
	msg.Event = EventRecovered
	if !strings.Contains(Text(msg), "receiving pings again") {
		t.Fatal(Text(msg))
	}
}

func TestGuardRefusesDemo(t *testing.T) {
	err := Guard{Demo: true, Next: Multi{}}.Send(context.Background(), model.Channel{}, sampleMessage())
	if err != ErrDemoDisabled {
		t.Fatal(err)
	}
}

func TestSlackAndWebhook(t *testing.T) {
	var slackBody, hookBody []byte
	slackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slackBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer slackSrv.Close()
	hookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		hookBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer hookSrv.Close()

	client := slackSrv.Client()
	multi := Multi{Slack: Slack{Client: client}, Hook: Webhook{Client: client}}
	msg := sampleMessage()
	slackCfg, _ := json.Marshal(SlackConfig{WebhookURL: slackSrv.URL})
	if err := multi.Send(context.Background(), model.Channel{Kind: model.KindSlack, Config: slackCfg}, msg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(slackBody), `"text"`) || strings.Contains(string(slackBody), "secret") && strings.Contains(string(slackBody), msg.Check.PingToken) {
		t.Fatal(string(slackBody))
	}
	hookCfg, _ := json.Marshal(WebhookConfig{URL: hookSrv.URL, Headers: map[string]string{"Authorization": "Bearer t"}})
	if err := multi.Send(context.Background(), model.Channel{Kind: model.KindWebhook, Config: hookCfg}, msg); err != nil {
		t.Fatal(err)
	}
	var payload Payload
	if err := json.Unmarshal(hookBody, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Event != EventDown || payload.Check.Name != "nightly-backup" || payload.LastPingAt == nil {
		t.Fatalf("%+v", payload)
	}
}

func TestValidateChannel(t *testing.T) {
	raw, _ := json.Marshal(EmailConfig{To: []string{"ops@example.com"}})
	if err := Validate(model.KindEmail, raw); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(EmailConfig{To: []string{"not-an-email"}})
	if err := Validate(model.KindEmail, raw); err == nil {
		t.Fatal("expected invalid email")
	}
	raw, _ = json.Marshal(WebhookConfig{URL: "ftp://example.com"})
	if err := Validate(model.KindWebhook, raw); err == nil {
		t.Fatal("expected bad url")
	}
}
