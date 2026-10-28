package app

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rejisterjack/tock/internal/model"
	"github.com/rejisterjack/tock/internal/notify"
	"github.com/rejisterjack/tock/internal/store"
	"github.com/rejisterjack/tock/internal/store/mem"
)

type fakeSender struct {
	mu     sync.Mutex
	events []string
}

func (f *fakeSender) Send(_ context.Context, _ model.Channel, msg notify.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, msg.Event)
	return nil
}

func (f *fakeSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func newSvc(t *testing.T, now time.Time) (*Service, *mem.Mem, *fakeSender) {
	t.Helper()
	repo := mem.New()
	sender := &fakeSender{}
	svc := &Service{
		Repo:      repo,
		Notifier:  sender,
		BaseURL:   "http://localhost:8080",
		Retention: 90 * 24 * time.Hour,
		Now:       func() time.Time { return now },
	}
	return svc, repo, sender
}

func emailChannel(t *testing.T, svc *Service) {
	t.Helper()
	raw, err := json.Marshal(notify.EmailConfig{To: []string{"ops@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	on := true
	if _, err := svc.CreateChannel(context.Background(), ChannelInput{
		Kind: model.KindEmail, Name: "ops", Config: raw, Enabled: &on,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTickDownOnceAndRecovery(t *testing.T) {
	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	now := start
	svc, repo, sender := newSvc(t, now)
	svc.Now = func() time.Time { return now }
	emailChannel(t, svc)
	c, err := svc.CreateCheck(context.Background(), CheckInput{Name: "backup", PeriodSeconds: 60, GraceSeconds: 0})
	if err != nil {
		t.Fatal(err)
	}

	now = start.Add(30 * time.Second)
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 0 {
		t.Fatal("inside the period should not alert")
	}

	now = start.Add(61 * time.Second)
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 1 || sender.events[0] != notify.EventDown {
		t.Fatalf("events %#v", sender.events)
	}
	got, err := repo.GetCheck(context.Background(), c.ID)
	if err != nil || got.Status != model.StatusDown {
		t.Fatalf("%+v %v", got, err)
	}

	now = start.Add(2 * time.Minute)
	if err := svc.Ping(context.Background(), c.PingToken, "127.0.0.1", "curl"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Ping(context.Background(), c.PingToken, "127.0.0.1", "curl"); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 2 || sender.events[1] != notify.EventRecovered {
		t.Fatalf("events %#v", sender.events)
	}
	alerts, err := svc.ListAlerts(context.Background(), c.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 2 {
		t.Fatalf("alerts %d", len(alerts))
	}
}

func TestGraceWindowDoesNotAlert(t *testing.T) {
	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	now := start
	svc, _, sender := newSvc(t, now)
	svc.Now = func() time.Time { return now }
	emailChannel(t, svc)
	if _, err := svc.CreateCheck(context.Background(), CheckInput{Name: "backup", PeriodSeconds: 60, GraceSeconds: 120}); err != nil {
		t.Fatal(err)
	}
	now = start.Add(90 * time.Second)
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 0 {
		t.Fatal("grace window alerted")
	}
	now = start.Add(181 * time.Second)
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 1 {
		t.Fatalf("count %d", sender.count())
	}
}

func TestPausedPingDoesNotRecover(t *testing.T) {
	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	now := start
	svc, _, sender := newSvc(t, now)
	svc.Now = func() time.Time { return now }
	emailChannel(t, svc)
	c, err := svc.CreateCheck(context.Background(), CheckInput{Name: "backup", PeriodSeconds: 60, GraceSeconds: 0})
	if err != nil {
		t.Fatal(err)
	}
	now = start.Add(61 * time.Second)
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Pause(context.Background(), c.ID); err != nil {
		t.Fatal(err)
	}
	now = start.Add(70 * time.Second)
	if err := svc.Ping(context.Background(), c.PingToken, "10.0.0.1", "curl"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetCheck(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.StatusDown || got.LastPingAt == nil || sender.count() != 1 {
		t.Fatalf("status %s events %d ping %v", got.Status, sender.count(), got.LastPingAt)
	}
	resumed, err := svc.Resume(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != model.StatusUp || resumed.Paused || sender.count() != 1 {
		t.Fatalf("resume status %s paused %v events %d", resumed.Status, resumed.Paused, sender.count())
	}
}

func TestNoChannelRecordsFailure(t *testing.T) {
	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	now := start
	svc, _, sender := newSvc(t, now)
	svc.Now = func() time.Time { return now }
	c, err := svc.CreateCheck(context.Background(), CheckInput{Name: "backup", PeriodSeconds: 60, GraceSeconds: 0})
	if err != nil {
		t.Fatal(err)
	}
	now = start.Add(2 * time.Minute)
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 0 {
		t.Fatal("sender should not be called")
	}
	alerts, err := svc.ListAlerts(context.Background(), c.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0].Status != model.AlertFailed {
		t.Fatalf("%+v", alerts)
	}
}

func TestRetentionDeletesOldPings(t *testing.T) {
	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	now := start
	svc, repo, _ := newSvc(t, now)
	svc.Now = func() time.Time { return now }
	svc.Retention = 24 * time.Hour
	c, err := svc.CreateCheck(context.Background(), CheckInput{Name: "backup", PeriodSeconds: 86400, GraceSeconds: 0})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertPing(context.Background(), model.Ping{CheckID: c.ID, ReceivedAt: start.Add(-48 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertPing(context.Background(), model.Ping{CheckID: c.ID, ReceivedAt: start.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	pings, err := svc.ListPings(context.Background(), c.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pings) != 1 {
		t.Fatalf("pings %d", len(pings))
	}
}

func TestUnknownPing(t *testing.T) {
	svc, _, _ := newSvc(t, time.Now())
	err := svc.Ping(context.Background(), "missing", "", "")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestDemoSkipsWritesAndAlerts(t *testing.T) {
	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	now := start
	svc, repo, sender := newSvc(t, now)
	svc.Now = func() time.Time { return now }
	svc.Demo = true
	svc.Notifier = notify.Guard{Demo: true, Next: sender}
	c := model.Check{
		ID: "c1", Name: "backup", PingToken: "tok", PeriodSeconds: 60, GraceSeconds: 0,
		Status: model.StatusUp, CreatedAt: start.Add(-time.Hour), UpdatedAt: start,
	}
	if err := repo.CreateCheck(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if err := svc.Ping(context.Background(), "tok", "1.1.1.1", "curl"); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetCheck(context.Background(), "c1")
	if got.LastPingAt != nil {
		t.Fatal("demo ping should not write")
	}
	now = start.Add(48 * time.Hour)
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetCheck(context.Background(), "c1")
	if got.Status != model.StatusUp || sender.count() != 0 {
		t.Fatalf("status %s events %d", got.Status, sender.count())
	}
	if _, err := svc.CreateCheck(context.Background(), CheckInput{Name: "x", PeriodSeconds: 60, GraceSeconds: 0}); !errors.Is(err, ErrReadOnly) {
		t.Fatal(err)
	}
}

func TestPendingSurvivesUntilDelivered(t *testing.T) {
	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	now := start
	svc, repo, sender := newSvc(t, now)
	svc.Now = func() time.Time { return now }
	emailChannel(t, svc)
	c, err := svc.CreateCheck(context.Background(), CheckInput{Name: "backup", PeriodSeconds: 60, GraceSeconds: 0})
	if err != nil {
		t.Fatal(err)
	}
	now = start.Add(2 * time.Minute)
	channels, err := repo.ListEnabledChannels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.EnqueueDown(context.Background(), now, channels); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 0 {
		t.Fatal("enqueue must not send")
	}
	alerts, err := svc.ListAlerts(context.Background(), c.ID, 10)
	if err != nil || len(alerts) != 1 || alerts[0].Status != model.AlertPending {
		t.Fatalf("%+v %v", alerts, err)
	}
	if err := svc.deliverPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.deliverPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 1 || sender.events[0] != notify.EventDown {
		t.Fatalf("%#v", sender.events)
	}
	alerts, err = svc.ListAlerts(context.Background(), c.ID, 10)
	if err != nil || alerts[0].Status != model.AlertSent {
		t.Fatalf("%+v %v", alerts, err)
	}
}

type errSender struct{ calls int }

func (e *errSender) Send(context.Context, model.Channel, notify.Message) error {
	e.calls++
	return errors.New("smtp down")
}

func TestFailedAlertIsNotRetried(t *testing.T) {
	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	now := start
	repo := mem.New()
	sender := &errSender{}
	svc := &Service{
		Repo: repo, Notifier: sender, BaseURL: "http://localhost:8080",
		Now: func() time.Time { return now },
	}
	on := true
	raw, _ := json.Marshal(notify.EmailConfig{To: []string{"ops@example.com"}})
	if _, err := svc.CreateChannel(context.Background(), ChannelInput{Kind: model.KindEmail, Name: "ops", Config: raw, Enabled: &on}); err != nil {
		t.Fatal(err)
	}
	c, err := svc.CreateCheck(context.Background(), CheckInput{Name: "backup", PeriodSeconds: 60, GraceSeconds: 0})
	if err != nil {
		t.Fatal(err)
	}
	now = start.Add(2 * time.Minute)
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 {
		t.Fatalf("calls %d", sender.calls)
	}
	alerts, err := svc.ListAlerts(context.Background(), c.ID, 10)
	if err != nil || len(alerts) != 1 || alerts[0].Status != model.AlertFailed || alerts[0].Error != "smtp down" {
		t.Fatalf("%+v %v", alerts, err)
	}
}
