// Package notify delivers down and recovery alerts.
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/rejisterjack/tock/internal/check"
	"github.com/rejisterjack/tock/internal/model"
)

const (
	// EventDown is sent when a check becomes overdue.
	EventDown = model.AlertDown
	// EventRecovered is sent when a down check pings again.
	EventRecovered = model.AlertRecovered
)

// ErrDemoDisabled is returned when outbound notifications are refused.
var ErrDemoDisabled = errors.New("notifications are disabled in demo mode")

// Message is one alert to deliver.
type Message struct {
	Event        string
	Check        model.Check
	SentAt       time.Time
	DashboardURL string
}

// Sender delivers a message to one channel.
type Sender interface {
	Send(ctx context.Context, ch model.Channel, msg Message) error
}

// Guard refuses to send when Demo is set, then delegates.
type Guard struct {
	Demo bool
	Next Sender
}

// Send implements Sender.
func (g Guard) Send(ctx context.Context, ch model.Channel, msg Message) error {
	if g.Demo {
		return ErrDemoDisabled
	}
	if g.Next == nil {
		return errors.New("no notifier configured")
	}
	return g.Next.Send(ctx, ch, msg)
}

// Multi routes a channel to the sender for its kind.
type Multi struct {
	Email Email
	Slack Slack
	Hook  Webhook
}

// Send implements Sender.
func (m Multi) Send(ctx context.Context, ch model.Channel, msg Message) error {
	switch ch.Kind {
	case model.KindEmail:
		return m.Email.Send(ctx, ch, msg)
	case model.KindSlack:
		return m.Slack.Send(ctx, ch, msg)
	case model.KindWebhook:
		return m.Hook.Send(ctx, ch, msg)
	default:
		return fmt.Errorf("unknown channel kind %q", ch.Kind)
	}
}

// EmailConfig is the JSON stored for an email channel.
type EmailConfig struct {
	To []string `json:"to"`
}

// SlackConfig is the JSON stored for a Slack incoming webhook.
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// WebhookConfig is the JSON stored for a generic webhook.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// Validate checks a channel kind and its config JSON.
func Validate(kind string, raw json.RawMessage) error {
	switch kind {
	case model.KindEmail:
		cfg, err := ParseEmail(raw)
		if err != nil {
			return err
		}
		if len(cfg.To) == 0 {
			return errors.New("email channel needs at least one recipient")
		}
		for _, addr := range cfg.To {
			if _, err := mail.ParseAddress(addr); err != nil {
				return fmt.Errorf("invalid email %q", addr)
			}
		}
		return nil
	case model.KindSlack:
		cfg, err := ParseSlack(raw)
		if err != nil {
			return err
		}
		return validateURL(cfg.WebhookURL)
	case model.KindWebhook:
		cfg, err := ParseWebhook(raw)
		if err != nil {
			return err
		}
		if err := validateURL(cfg.URL); err != nil {
			return err
		}
		if len(cfg.Headers) > 20 {
			return errors.New("webhook channels accept at most 20 headers")
		}
		for k, v := range cfg.Headers {
			if !validHeaderName(k) || strings.ContainsAny(v, "\r\n") {
				return fmt.Errorf("invalid webhook header %q", k)
			}
		}
		return nil
	default:
		return errors.New("channel kind must be email, slack, or webhook")
	}
}

// ParseEmail decodes an email channel config.
func ParseEmail(raw json.RawMessage) (EmailConfig, error) {
	var cfg EmailConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return EmailConfig{}, errors.New("email config must be JSON")
	}
	var to []string
	for _, addr := range cfg.To {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			to = append(to, addr)
		}
	}
	cfg.To = to
	return cfg, nil
}

// ParseSlack decodes a Slack channel config.
func ParseSlack(raw json.RawMessage) (SlackConfig, error) {
	var cfg SlackConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return SlackConfig{}, errors.New("slack config must be JSON")
	}
	cfg.WebhookURL = strings.TrimSpace(cfg.WebhookURL)
	return cfg, nil
}

// ParseWebhook decodes a generic webhook config.
func ParseWebhook(raw json.RawMessage) (WebhookConfig, error) {
	var cfg WebhookConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return WebhookConfig{}, errors.New("webhook config must be JSON")
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	return cfg, nil
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("notification URL must be absolute http or https")
	}
	return nil
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	switch strings.ToLower(name) {
	case "host", "content-length", "transfer-encoding", "connection":
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("!#$%&'*+-.^_`|~", r) {
			continue
		}
		return false
	}
	return true
}

// Subject is the alert subject line.
func Subject(msg Message) string {
	switch msg.Event {
	case EventRecovered:
		return "Tock: " + msg.Check.Name + " recovered"
	default:
		return "Tock: " + msg.Check.Name + " is down"
	}
}

// Text is the plain-text alert body. It never includes the ping token.
func Text(msg Message) string {
	last := "never"
	if msg.Check.LastPingAt != nil {
		last = msg.Check.LastPingAt.UTC().Format(time.RFC3339)
	}
	what := "missed its expected ping"
	if msg.Event == EventRecovered {
		what = "is receiving pings again"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tock: %s %s.\n\n", msg.Check.Name, what)
	fmt.Fprintf(&b, "Last ping: %s\n", last)
	fmt.Fprintf(&b, "Expected every: %s\n", check.FormatInterval(msg.Check.PeriodSeconds))
	fmt.Fprintf(&b, "Grace: %s\n", check.FormatInterval(msg.Check.GraceSeconds))
	if msg.DashboardURL != "" {
		fmt.Fprintf(&b, "Dashboard: %s\n", msg.DashboardURL)
	}
	return b.String()
}

// Payload is the JSON body posted to a generic webhook.
type Payload struct {
	Event      string       `json:"event"`
	Check      PayloadCheck `json:"check"`
	LastPingAt *time.Time   `json:"last_ping_at"`
	SentAt     time.Time    `json:"sent_at"`
}

// PayloadCheck is the check object inside a webhook payload.
type PayloadCheck struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	PeriodSeconds int    `json:"period_seconds"`
	GraceSeconds  int    `json:"grace_seconds"`
	Paused        bool   `json:"paused"`
}

// WebhookPayload builds the outbound JSON document.
func WebhookPayload(msg Message) Payload {
	return Payload{
		Event: msg.Event,
		Check: PayloadCheck{
			ID:            msg.Check.ID,
			Name:          msg.Check.Name,
			Status:        msg.Check.Status,
			PeriodSeconds: msg.Check.PeriodSeconds,
			GraceSeconds:  msg.Check.GraceSeconds,
			Paused:        msg.Check.Paused,
		},
		LastPingAt: msg.Check.LastPingAt,
		SentAt:     msg.SentAt.UTC(),
	}
}
