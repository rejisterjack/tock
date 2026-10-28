//go:build integration

package app

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rejisterjack/tock/internal/db"
	"github.com/rejisterjack/tock/internal/demo"
	"github.com/rejisterjack/tock/internal/model"
	"github.com/rejisterjack/tock/internal/notify"
	"github.com/rejisterjack/tock/internal/store/pg"
)

func TestMain(m *testing.M) {
	if os.Getenv("DATABASE_URL") == "" {
		os.Exit(m.Run())
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, os.Getenv("DATABASE_URL")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestIntegrationAlertsAndRetention(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repo := pg.New(pool)
	truncate(t, pool)

	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	now := start
	sender := &fakeSender{}
	svc := &Service{
		Repo: repo, Notifier: sender, BaseURL: "http://localhost:8080",
		Retention: 24 * time.Hour, Now: func() time.Time { return now },
	}
	raw := []byte(`{"to":["ops@example.com"]}`)
	on := true
	if _, err := svc.CreateChannel(ctx, ChannelInput{Kind: model.KindEmail, Name: "ops", Config: raw, Enabled: &on}); err != nil {
		t.Fatal(err)
	}
	created, err := svc.CreateCheck(ctx, CheckInput{Name: "backup", PeriodSeconds: 86400, GraceSeconds: 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE checks SET created_at = $2, updated_at = $2 WHERE id = $1`, created.ID, start.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	alerts, err := svc.ListAlerts(ctx, created.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0].Kind != notify.EventDown || sender.count() != 1 {
		t.Fatalf("alerts %+v events %d", alerts, sender.count())
	}

	if err := svc.Ping(ctx, created.PingToken, "127.0.0.1", "curl"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Ping(ctx, created.PingToken, "127.0.0.1", "curl"); err != nil {
		t.Fatal(err)
	}
	alerts, err = svc.ListAlerts(ctx, created.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 || sender.count() != 2 {
		t.Fatalf("after recovery alerts %d events %d", len(alerts), sender.count())
	}
	got, err := svc.GetCheck(ctx, created.ID)
	if err != nil || got.Status != model.StatusUp {
		t.Fatalf("%+v %v", got, err)
	}

	if err := repo.InsertPing(ctx, model.Ping{CheckID: created.ID, ReceivedAt: start.Add(-72 * time.Hour), RemoteIP: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	pings, err := svc.ListPings(ctx, created.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pings {
		if p.ReceivedAt.Before(start.Add(-24 * time.Hour)) {
			t.Fatalf("old ping kept: %+v", p)
		}
	}
	if len(pings) < 2 {
		t.Fatalf("recent pings missing: %d", len(pings))
	}
	if sender.count() != 2 {
		t.Fatalf("tick after recovery notified again: %d", sender.count())
	}

	fresh, err := svc.CreateCheck(ctx, CheckInput{Name: "within-grace", PeriodSeconds: 60, GraceSeconds: 3600})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	fresh, err = svc.GetCheck(ctx, fresh.ID)
	if err != nil || fresh.Status != model.StatusNew {
		t.Fatalf("grace check %+v %v", fresh, err)
	}

	paused, err := svc.CreateCheck(ctx, CheckInput{Name: "paused", PeriodSeconds: 60, GraceSeconds: 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE checks SET created_at = $2, updated_at = $2 WHERE id = $1`, paused.ID, start.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	before := sender.count()
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count() != before+1 {
		t.Fatalf("paused setup events %d", sender.count())
	}
	if _, err := svc.Pause(ctx, paused.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Ping(ctx, paused.PingToken, "10.0.0.8", "curl"); err != nil {
		t.Fatal(err)
	}
	got, err = svc.GetCheck(ctx, paused.ID)
	if err != nil || got.Status != model.StatusDown || !got.Paused {
		t.Fatalf("paused ping %+v %v", got, err)
	}
	got, err = svc.Resume(ctx, paused.ID)
	if err != nil || got.Status != model.StatusUp || got.Paused || sender.count() != before+1 {
		t.Fatalf("resume %+v events %d err %v", got, sender.count(), err)
	}
}

func TestIntegrationDemoSeed(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	truncate(t, pool)
	repo := pg.New(pool)
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	if err := demo.Seed(ctx, repo, now); err != nil {
		t.Fatal(err)
	}
	if err := demo.Maintain(ctx, repo, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	checks, err := repo.ListChecks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 3 {
		t.Fatalf("seeded %d checks", len(checks))
	}
	if err := demo.Seed(ctx, repo, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	checks, err = repo.ListChecks(ctx)
	if err != nil || len(checks) != 3 {
		t.Fatalf("reseed %d %v", len(checks), err)
	}
}

func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE TABLE alerts, pings, channels, checks RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
}
