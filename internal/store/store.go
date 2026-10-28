// Package store is the persistence boundary used by the application service.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/rejisterjack/tock/internal/model"
)

var (
	// ErrNotFound means the row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means a unique key collided.
	ErrConflict = errors.New("conflict")
)

// Repo is the storage used by the service. The Postgres and memory
// implementations share these transitions so tests and production agree.
type Repo interface {
	Ping(ctx context.Context) error

	CreateCheck(ctx context.Context, c model.Check) error
	UpdateCheck(ctx context.Context, id, name string, period, grace int, now time.Time) (model.Check, error)
	GetCheck(ctx context.Context, id string) (model.Check, error)
	GetCheckByToken(ctx context.Context, token string) (model.Check, error)
	ListChecks(ctx context.Context) ([]model.Check, error)
	DeleteCheck(ctx context.Context, id string) error

	ApplyPing(ctx context.Context, token string, at time.Time, ip, ua string, channels []model.Channel) (model.PingResult, error)
	Pause(ctx context.Context, id string, now time.Time) (model.Check, error)
	Resume(ctx context.Context, id string, now time.Time) (model.Check, error)
	ClaimDue(ctx context.Context, now time.Time) ([]model.Check, error)
	// EnqueueDown marks overdue checks down and inserts alert rows in the same
	// step, so a crash cannot leave a check down with no alert recorded.
	EnqueueDown(ctx context.Context, now time.Time, channels []model.Channel) error
	ListPending(ctx context.Context) ([]Pending, error)
	SettleAlert(ctx context.Context, id, status, errText string, at time.Time) error

	CreateChannel(ctx context.Context, c model.Channel) error
	UpdateChannel(ctx context.Context, c model.Channel) error
	DeleteChannel(ctx context.Context, id string) error
	GetChannel(ctx context.Context, id string) (model.Channel, error)
	ListChannels(ctx context.Context) ([]model.Channel, error)
	ListEnabledChannels(ctx context.Context) ([]model.Channel, error)

	InsertAlert(ctx context.Context, a model.Alert) error
	ListAlerts(ctx context.Context, checkID string, limit int) ([]model.Alert, error)
	ListPings(ctx context.Context, checkID string, limit int) ([]model.Ping, error)
	InsertPing(ctx context.Context, p model.Ping) error
	DeletePingsBefore(ctx context.Context, before time.Time) (int64, error)

	// WithLock runs fn while holding the scheduler lock. acquired is false when
	// another replica holds it; fn is not called in that case.
	WithLock(ctx context.Context, fn func(context.Context) error) (acquired bool, err error)

	// RestoreDemoCheck forces a synthetic check back to a canonical status and
	// moves its latest ping to lastPing so the public demo does not drift.
	RestoreDemoCheck(ctx context.Context, id, status string, lastPing, now time.Time) error
}
