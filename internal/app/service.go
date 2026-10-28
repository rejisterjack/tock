// Package app is the check, ping, and alert workflow.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/rejisterjack/tock/internal/check"
	"github.com/rejisterjack/tock/internal/model"
	"github.com/rejisterjack/tock/internal/notify"
	"github.com/rejisterjack/tock/internal/store"
	"time"
)

// ErrReadOnly is returned when a public demo refuses a mutation.
var ErrReadOnly = errors.New("demo mode is read-only")

// CheckInput is the operator-supplied schedule for a check.
type CheckInput struct {
	Name          string
	PeriodSeconds int
	GraceSeconds  int
}

// ChannelInput is the operator-supplied notification channel.
// A nil Enabled means "on" when creating and "leave unchanged" when updating.
type ChannelInput struct {
	Kind    string
	Name    string
	Config  json.RawMessage
	Enabled *bool
}

// Service coordinates storage and notifications.
type Service struct {
	Repo      store.Repo
	Notifier  notify.Sender
	BaseURL   string
	Demo      bool
	Retention time.Duration
	Now       func() time.Time
	Log       *slog.Logger
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// CreateCheck stores a new check and generates its ping token.
func (s *Service) CreateCheck(ctx context.Context, in CheckInput) (model.Check, error) {
	if s.Demo {
		return model.Check{}, ErrReadOnly
	}
	in.Name = strings.TrimSpace(in.Name)
	if err := check.Validate(in.Name, in.PeriodSeconds, in.GraceSeconds); err != nil {
		return model.Check{}, err
	}
	var last error
	for i := 0; i < 3; i++ {
		id, err := check.NewID()
		if err != nil {
			return model.Check{}, err
		}
		token, err := check.NewToken()
		if err != nil {
			return model.Check{}, err
		}
		now := s.now()
		c := model.Check{
			ID:            id,
			Name:          in.Name,
			PingToken:     token,
			PeriodSeconds: in.PeriodSeconds,
			GraceSeconds:  in.GraceSeconds,
			Status:        model.StatusNew,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		err = s.Repo.CreateCheck(ctx, c)
		if err == nil {
			return c, nil
		}
		if errors.Is(err, store.ErrConflict) {
			last = err
			continue
		}
		return model.Check{}, err
	}
	return model.Check{}, last
}

// UpdateCheck changes the name and schedule. The ping token stays the same.
func (s *Service) UpdateCheck(ctx context.Context, id string, in CheckInput) (model.Check, error) {
	if s.Demo {
		return model.Check{}, ErrReadOnly
	}
	in.Name = strings.TrimSpace(in.Name)
	if err := check.Validate(in.Name, in.PeriodSeconds, in.GraceSeconds); err != nil {
		return model.Check{}, err
	}
	return s.Repo.UpdateCheck(ctx, id, in.Name, in.PeriodSeconds, in.GraceSeconds, s.now())
}

// GetCheck loads one check.
func (s *Service) GetCheck(ctx context.Context, id string) (model.Check, error) {
	return s.Repo.GetCheck(ctx, id)
}

// ListChecks returns checks newest first.
func (s *Service) ListChecks(ctx context.Context) ([]model.Check, error) {
	return s.Repo.ListChecks(ctx)
}

// DeleteCheck removes a check and its history.
func (s *Service) DeleteCheck(ctx context.Context, id string) error {
	if s.Demo {
		return ErrReadOnly
	}
	return s.Repo.DeleteCheck(ctx, id)
}

// Pause stops scheduling until Resume.
func (s *Service) Pause(ctx context.Context, id string) (model.Check, error) {
	if s.Demo {
		return model.Check{}, ErrReadOnly
	}
	return s.Repo.Pause(ctx, id, s.now())
}

// Resume clears pause. A check that was pinged while paused and is still
// inside its window returns to up or new without a recovery notification.
func (s *Service) Resume(ctx context.Context, id string) (model.Check, error) {
	if s.Demo {
		return model.Check{}, ErrReadOnly
	}
	return s.Repo.Resume(ctx, id, s.now())
}

// Ping records a success signal. In demo mode the token is checked and the
// write is skipped so synthetic jobs stay put.
func (s *Service) Ping(ctx context.Context, token, ip, ua string) error {
	if s.Demo {
		_, err := s.Repo.GetCheckByToken(ctx, token)
		return err
	}
	channels, err := s.Repo.ListEnabledChannels(ctx)
	if err != nil {
		return err
	}
	if _, err := s.Repo.ApplyPing(ctx, token, s.now(), clip(ip, 64), clip(ua, 300), channels); err != nil {
		return err
	}
	return s.deliverPending(ctx)
}

// Tick claims overdue checks, notifies once, and purges old pings.
// Demo mode only purges; the simulator owns synthetic status.
func (s *Service) Tick(ctx context.Context) error {
	acquired, err := s.Repo.WithLock(ctx, func(ctx context.Context) error {
		if err := s.purge(ctx); err != nil {
			return err
		}
		if s.Demo {
			return nil
		}
		channels, err := s.Repo.ListEnabledChannels(ctx)
		if err != nil {
			return err
		}
		if err := s.Repo.EnqueueDown(ctx, s.now(), channels); err != nil {
			return err
		}
		return s.deliverPending(ctx)
	})
	if err != nil {
		return err
	}
	if !acquired {
		s.log().Debug("scheduler lock busy")
	}
	return nil
}

func (s *Service) purge(ctx context.Context) error {
	if s.Retention <= 0 {
		return nil
	}
	_, err := s.Repo.DeletePingsBefore(ctx, s.now().Add(-s.Retention))
	return err
}

func (s *Service) deliverPending(ctx context.Context) error {
	rows, err := s.Repo.ListPending(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		msg := notify.Message{
			Event:        row.Alert.Kind,
			Check:        row.Check,
			SentAt:       s.now(),
			DashboardURL: strings.TrimRight(s.BaseURL, "/") + "/checks/" + row.Check.ID,
		}
		var sendErr error
		if s.Notifier == nil {
			sendErr = errors.New("no notifier configured")
		} else {
			sendErr = s.Notifier.Send(ctx, row.Channel, msg)
		}
		status := model.AlertSent
		errText := ""
		if sendErr != nil {
			status = model.AlertFailed
			errText = sendErr.Error()
			s.log().Error("notify", "err", sendErr, "check_id", row.Check.ID, "channel_id", row.Channel.ID, "event", row.Alert.Kind)
		}
		if err := s.Repo.SettleAlert(ctx, row.Alert.ID, status, errText, s.now()); err != nil {
			s.log().Error("settle alert", "err", err, "alert_id", row.Alert.ID)
		}
	}
	return nil
}

// CreateChannel stores a notification channel.
func (s *Service) CreateChannel(ctx context.Context, in ChannelInput) (model.Channel, error) {
	if s.Demo {
		return model.Channel{}, ErrReadOnly
	}
	ch, err := s.prepareChannel("", in)
	if err != nil {
		return model.Channel{}, err
	}
	id, err := check.NewID()
	if err != nil {
		return model.Channel{}, err
	}
	now := s.now()
	ch.ID = id
	ch.CreatedAt = now
	ch.UpdatedAt = now
	if err := s.Repo.CreateChannel(ctx, ch); err != nil {
		return model.Channel{}, err
	}
	return ch, nil
}

// UpdateChannel replaces the name, config, and enabled flag.
func (s *Service) UpdateChannel(ctx context.Context, id string, in ChannelInput) (model.Channel, error) {
	if s.Demo {
		return model.Channel{}, ErrReadOnly
	}
	existing, err := s.Repo.GetChannel(ctx, id)
	if err != nil {
		return model.Channel{}, err
	}
	in.Kind = existing.Kind
	if in.Enabled == nil {
		current := existing.Enabled
		in.Enabled = &current
	}
	ch, err := s.prepareChannel(id, in)
	if err != nil {
		return model.Channel{}, err
	}
	ch.CreatedAt = existing.CreatedAt
	ch.UpdatedAt = s.now()
	if err := s.Repo.UpdateChannel(ctx, ch); err != nil {
		return model.Channel{}, err
	}
	return ch, nil
}

// DeleteChannel removes a channel. Past alerts keep the channel name.
func (s *Service) DeleteChannel(ctx context.Context, id string) error {
	if s.Demo {
		return ErrReadOnly
	}
	return s.Repo.DeleteChannel(ctx, id)
}

// ListChannels returns every channel.
func (s *Service) ListChannels(ctx context.Context) ([]model.Channel, error) {
	return s.Repo.ListChannels(ctx)
}

// GetChannel loads one channel.
func (s *Service) GetChannel(ctx context.Context, id string) (model.Channel, error) {
	return s.Repo.GetChannel(ctx, id)
}

// ListPings returns recent pings for a check.
func (s *Service) ListPings(ctx context.Context, checkID string, limit int) ([]model.Ping, error) {
	if _, err := s.Repo.GetCheck(ctx, checkID); err != nil {
		return nil, err
	}
	return s.Repo.ListPings(ctx, checkID, clampLimit(limit))
}

// ListAlerts returns recent alerts for a check.
func (s *Service) ListAlerts(ctx context.Context, checkID string, limit int) ([]model.Alert, error) {
	if _, err := s.Repo.GetCheck(ctx, checkID); err != nil {
		return nil, err
	}
	return s.Repo.ListAlerts(ctx, checkID, clampLimit(limit))
}

func (s *Service) prepareChannel(id string, in ChannelInput) (model.Channel, error) {
	in.Name = strings.TrimSpace(in.Name)
	if err := check.ValidateName(in.Name); err != nil {
		return model.Channel{}, err
	}
	if len(in.Config) == 0 {
		in.Config = json.RawMessage(`{}`)
	}
	if err := notify.Validate(in.Kind, in.Config); err != nil {
		return model.Channel{}, &check.ValidationError{Message: err.Error()}
	}
	// Re-marshal so stored JSON is normalized (trimmed addresses and URLs).
	norm, err := normalizeConfig(in.Kind, in.Config)
	if err != nil {
		return model.Channel{}, err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return model.Channel{
		ID:      id,
		Kind:    in.Kind,
		Name:    in.Name,
		Config:  norm,
		Enabled: enabled,
	}, nil
}

func normalizeConfig(kind string, raw json.RawMessage) (json.RawMessage, error) {
	switch kind {
	case model.KindEmail:
		cfg, err := notify.ParseEmail(raw)
		if err != nil {
			return nil, err
		}
		return json.Marshal(cfg)
	case model.KindSlack:
		cfg, err := notify.ParseSlack(raw)
		if err != nil {
			return nil, err
		}
		return json.Marshal(cfg)
	case model.KindWebhook:
		cfg, err := notify.ParseWebhook(raw)
		if err != nil {
			return nil, err
		}
		return json.Marshal(cfg)
	default:
		return raw, nil
	}
}

func clampLimit(n int) int {
	if n <= 0 {
		return 50
	}
	if n > 200 {
		return 200
	}
	return n
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}
