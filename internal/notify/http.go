package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rejisterjack/tock/internal/model"
)

// errNoRedirect is returned when an operator-configured URL answers with a
// redirect. Following it would send channel headers, including Authorization,
// to a different host.
var errNoRedirect = errors.New("redirects are not followed")

// Slack posts a text payload to an incoming webhook.
type Slack struct {
	Client *http.Client
}

// Webhook posts JSON to an operator-configured URL.
type Webhook struct {
	Client *http.Client
}

func (s Slack) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return defaultHTTPClient()
}

func (w Webhook) client() *http.Client {
	if w.Client != nil {
		return w.Client
	}
	return defaultHTTPClient()
}

func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errNoRedirect
		},
	}
}

// Send implements Sender.
func (s Slack) Send(ctx context.Context, ch model.Channel, msg Message) error {
	cfg, err := ParseSlack(ch.Config)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"text": strings.TrimSpace(Text(msg))})
	if err != nil {
		return err
	}
	return postJSON(ctx, s.client(), cfg.WebhookURL, body, nil)
}

// Send implements Sender.
func (w Webhook) Send(ctx context.Context, ch model.Channel, msg Message) error {
	cfg, err := ParseWebhook(ch.Config)
	if err != nil {
		return err
	}
	body, err := json.Marshal(WebhookPayload(msg))
	if err != nil {
		return err
	}
	return postJSON(ctx, w.client(), cfg.URL, body, cfg.Headers)
}

func postJSON(ctx context.Context, client *http.Client, endpoint string, body []byte, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Tock")
	for k, v := range headers {
		if strings.EqualFold(k, "content-type") {
			continue
		}
		req.Header.Set(k, v)
	}
	res, err := client.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("notification returned HTTP %d", res.StatusCode)
	}
	return nil
}
