package check

import (
	"testing"
	"time"

	"github.com/rejisterjack/tock/internal/model"
)

func TestTokensUnique(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != 32 {
			t.Fatalf("token length %d", len(tok))
		}
		if _, ok := seen[tok]; ok {
			t.Fatalf("duplicate token %s", tok)
		}
		seen[tok] = struct{}{}
	}
}

func TestValidatePeriod(t *testing.T) {
	if err := Validate("backup", 59, 0); err == nil {
		t.Fatal("expected period below 60s to fail")
	}
	if err := Validate("backup", 60, 0); err != nil {
		t.Fatal(err)
	}
	if err := Validate("backup", 60, -1); err == nil {
		t.Fatal("expected negative grace to fail")
	}
	if err := Validate("", 60, 0); err == nil {
		t.Fatal("expected empty name to fail")
	}
	if err := Validate("bad\nname", 60, 0); err == nil {
		t.Fatal("expected control character to fail")
	}
}

func TestDueAndDisplay(t *testing.T) {
	created := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	c := model.Check{
		Status:        model.StatusNew,
		PeriodSeconds: 3600,
		GraceSeconds:  600,
		CreatedAt:     created,
	}
	if !DueAt(c).Equal(created.Add(4200 * time.Second)) {
		t.Fatalf("due at %s", DueAt(c))
	}
	if DisplayStatus(c, created.Add(10*time.Minute)) != model.StatusNew {
		t.Fatal("fresh check should be new")
	}
	if DisplayStatus(c, created.Add(3601*time.Second)) != model.DisplayGrace {
		t.Fatal("new check inside grace should display grace")
	}
	if IsOverdue(c, created.Add(4200*time.Second)) {
		t.Fatal("exact due time is not overdue")
	}
	if !IsOverdue(c, created.Add(4201*time.Second)) {
		t.Fatal("past due should be overdue")
	}

	pinged := created.Add(time.Hour)
	c.Status = model.StatusUp
	c.LastPingAt = &pinged
	if DisplayStatus(c, pinged.Add(30*time.Minute)) != model.StatusUp {
		t.Fatal("recent ping should be up")
	}
	if DisplayStatus(c, pinged.Add(3601*time.Second)) != model.DisplayGrace {
		t.Fatal("late ping inside grace should display grace")
	}
	c.Status = model.StatusDown
	if DisplayStatus(c, pinged) != model.StatusDown {
		t.Fatal("stored down stays down")
	}
	c.Paused = true
	c.Status = model.StatusUp
	if DisplayStatus(c, pinged.Add(48*time.Hour)) != model.DisplayPaused {
		t.Fatal("paused wins over the clock")
	}
}

func TestNextAfterPingAndResume(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	c := model.Check{Status: model.StatusNew, PeriodSeconds: 60, GraceSeconds: 0, CreatedAt: now}
	status, recovery := NextAfterPing(c)
	if status != model.StatusUp || recovery {
		t.Fatalf("first ping: %s %v", status, recovery)
	}
	c.Status = model.StatusDown
	status, recovery = NextAfterPing(c)
	if status != model.StatusUp || !recovery {
		t.Fatalf("recovery ping: %s %v", status, recovery)
	}
	c.Paused = true
	c.Status = model.StatusDown
	status, recovery = NextAfterPing(c)
	if status != model.StatusDown || recovery {
		t.Fatalf("paused ping: %s %v", status, recovery)
	}

	c.Paused = true
	recent := now
	c.LastPingAt = &recent
	c.CreatedAt = now.Add(-time.Hour)
	status, resolve := NextAfterResume(c, now)
	if status != model.StatusUp || !resolve {
		t.Fatalf("resume after ping during pause: %s %v", status, resolve)
	}
	c.LastPingAt = nil
	c.CreatedAt = now
	status, resolve = NextAfterResume(c, now)
	if status != model.StatusNew || !resolve {
		t.Fatalf("resume never pinged: %s %v", status, resolve)
	}
	old := now.Add(-2 * time.Hour)
	c.LastPingAt = &old
	c.Status = model.StatusDown
	status, resolve = NextAfterResume(c, now)
	if status != model.StatusDown || resolve {
		t.Fatalf("resume still overdue: %s %v", status, resolve)
	}
	c.Status = model.StatusUp
	status, resolve = NextAfterResume(c, now)
	if status != model.StatusUp || resolve {
		t.Fatalf("resume overdue up check stays up for the scheduler: %s %v", status, resolve)
	}
}

func TestFormatAndSplitInterval(t *testing.T) {
	if FormatInterval(86400) != "1d" || FormatInterval(7200) != "2h" || FormatInterval(120) != "2m" {
		t.Fatalf("format: %s %s %s", FormatInterval(86400), FormatInterval(7200), FormatInterval(120))
	}
	if FormatInterval(90) != "90s" {
		t.Fatal(FormatInterval(90))
	}
	v, u := SplitInterval(172800)
	if v != 2 || u != "days" {
		t.Fatalf("%d %s", v, u)
	}
	sec, err := ParseInterval(2, "hours")
	if err != nil || sec != 7200 {
		t.Fatalf("%d %v", sec, err)
	}
}
