// Package demo keeps a public instance on synthetic jobs and never alerts.
package demo

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/rejisterjack/tock/internal/check"
	"github.com/rejisterjack/tock/internal/model"
	"github.com/rejisterjack/tock/internal/store"
)

type spec struct {
	ID       string
	Token    string
	Name     string
	Status   string
	Period   int
	Grace    int
	LastPing time.Duration
}

const vacuumID = "33333333-3333-4333-8333-333333333333"

var specs = []spec{
	{
		ID: "11111111-1111-4111-8111-111111111111", Token: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name: "nightly-backup", Status: model.StatusUp, Period: 24 * 3600, Grace: 3600, LastPing: -2 * time.Hour,
	},
	{
		ID: "22222222-2222-4222-8222-222222222222", Token: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Name: "weekly-digest", Status: model.StatusUp, Period: 24 * 3600, Grace: 2 * 3600, LastPing: -25 * time.Hour,
	},
	{
		ID: vacuumID, Token: "cccccccccccccccccccccccccccccccc",
		Name: "db-vacuum", Status: model.StatusDown, Period: 3600, Grace: 600, LastPing: -6 * time.Hour,
	},
}

// Seed inserts the synthetic checks once, then aligns their clocks.
func Seed(ctx context.Context, repo store.Repo, now time.Time) error {
	now = now.UTC()
	for _, sp := range specs {
		_, err := repo.GetCheck(ctx, sp.ID)
		if errors.Is(err, store.ErrNotFound) {
			last := now.Add(sp.LastPing)
			created := now.Add(-14 * 24 * time.Hour)
			if err := repo.CreateCheck(ctx, model.Check{
				ID: sp.ID, Name: sp.Name, PingToken: sp.Token,
				PeriodSeconds: sp.Period, GraceSeconds: sp.Grace, Status: sp.Status,
				LastPingAt: &last, CreatedAt: created, UpdatedAt: now,
			}); err != nil {
				return err
			}
			older := last.Add(-time.Duration(sp.Period) * time.Second)
			if err := repo.InsertPing(ctx, model.Ping{CheckID: sp.ID, ReceivedAt: older, RemoteIP: "demo", UserAgent: "tock-demo"}); err != nil {
				return err
			}
			if err := repo.InsertPing(ctx, model.Ping{CheckID: sp.ID, ReceivedAt: last, RemoteIP: "demo", UserAgent: "tock-demo"}); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	alerts, err := repo.ListAlerts(ctx, vacuumID, 1)
	if err != nil {
		return err
	}
	if len(alerts) == 0 {
		id, err := check.NewID()
		if err != nil {
			return err
		}
		if err := repo.InsertAlert(ctx, model.Alert{
			ID: id, CheckID: vacuumID, Kind: model.AlertDown, ChannelName: "example channel",
			SentAt: now.Add(-5 * time.Hour), Status: model.AlertSent,
		}); err != nil {
			return err
		}
	}
	return Maintain(ctx, repo, now)
}

// Maintain moves synthetic jobs back to their canonical clocks.
func Maintain(ctx context.Context, repo store.Repo, now time.Time) error {
	now = now.UTC()
	for _, sp := range specs {
		if err := repo.RestoreDemoCheck(ctx, sp.ID, sp.Status, now.Add(sp.LastPing), now); err != nil {
			return err
		}
	}
	return nil
}

// Run aligns the demo jobs until ctx is cancelled.
func Run(ctx context.Context, repo store.Repo, log *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := Maintain(ctx, repo, time.Now().UTC()); err != nil {
				log.Error("demo maintain", "err", err)
			}
		}
	}
}
