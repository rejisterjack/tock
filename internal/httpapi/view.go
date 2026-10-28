package httpapi

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/rejisterjack/tock/internal/check"
	"github.com/rejisterjack/tock/internal/model"
	"github.com/rejisterjack/tock/internal/notify"
)

// Page is the data passed to every dashboard template.
type Page struct {
	Title    string
	Version  string
	Demo     bool
	Authed   bool
	CSRF     string
	Nav      string
	Error    string
	Checks   []CheckView
	Check    CheckView
	Pings    []model.Ping
	Alerts   []model.Alert
	Channels []ChannelView
	Form     CheckForm
	Empty    bool
}

// CheckForm is the create and edit form.
type CheckForm struct {
	ID          string
	Name        string
	PeriodValue int
	PeriodUnit  string
	GraceValue  int
	GraceUnit   string
	Editing     bool
}

// CheckView is a check plus display fields.
type CheckView struct {
	model.Check
	DisplayStatus string
	PingURL       string
	Curl          string
	LastPingISO   string
	PeriodLabel   string
	GraceLabel    string
	PeriodValue   int
	PeriodUnit    string
	GraceValue    int
	GraceUnit     string
}

// ChannelView is a channel plus form fields.
type ChannelView struct {
	model.Channel
	To         string
	WebhookURL string
	URL        string
	Headers    string
}

func (s *Server) presentCheck(c model.Check, now time.Time) CheckView {
	v := CheckView{
		Check:         c,
		DisplayStatus: check.DisplayStatus(c, now),
		PingURL:       s.cfg.BaseURL + "/ping/" + c.PingToken,
		PeriodLabel:   check.FormatInterval(c.PeriodSeconds),
		GraceLabel:    check.FormatInterval(c.GraceSeconds),
	}
	v.Curl = "curl -fsS " + v.PingURL
	v.PeriodValue, v.PeriodUnit = check.SplitInterval(c.PeriodSeconds)
	v.GraceValue, v.GraceUnit = check.SplitInterval(c.GraceSeconds)
	if c.LastPingAt != nil {
		v.LastPingISO = c.LastPingAt.UTC().Format(time.RFC3339)
	}
	return v
}

func presentChannel(c model.Channel) ChannelView {
	v := ChannelView{Channel: c}
	switch c.Kind {
	case model.KindEmail:
		cfg, err := notify.ParseEmail(c.Config)
		if err == nil {
			v.To = strings.Join(cfg.To, ", ")
		}
	case model.KindSlack:
		cfg, err := notify.ParseSlack(c.Config)
		if err == nil {
			v.WebhookURL = cfg.WebhookURL
		}
	case model.KindWebhook:
		cfg, err := notify.ParseWebhook(c.Config)
		if err == nil {
			v.URL = cfg.URL
			v.Headers = formatHeaders(cfg.Headers)
		}
	}
	return v
}

func formatHeaders(h map[string]string) string {
	if len(h) == 0 {
		return ""
	}
	var b strings.Builder
	first := true
	for k, v := range h {
		if !first {
			b.WriteByte('\n')
		}
		first = false
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
	}
	return b.String()
}

func parseHeaders(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func splitRecipients(raw string) []string {
	raw = strings.ReplaceAll(raw, "\n", ",")
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// channelConfig builds JSON from a dashboard form.
func channelConfig(kind, to, slackURL, hookURL, headers string) (json.RawMessage, error) {
	var cfg any
	switch kind {
	case model.KindEmail:
		cfg = notify.EmailConfig{To: splitRecipients(to)}
	case model.KindSlack:
		cfg = notify.SlackConfig{WebhookURL: strings.TrimSpace(slackURL)}
	case model.KindWebhook:
		cfg = notify.WebhookConfig{URL: strings.TrimSpace(hookURL), Headers: parseHeaders(headers)}
	default:
		cfg = map[string]any{}
	}
	return json.Marshal(cfg)
}
