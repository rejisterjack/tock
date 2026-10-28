// Package mem is an in-memory Repo for tests and local HTTP checks.
package mem

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/rejisterjack/tock/internal/check"
	"github.com/rejisterjack/tock/internal/model"
	"github.com/rejisterjack/tock/internal/store"
)

// Mem stores checks, pings, channels, and alerts in process memory.
type Mem struct {
	mu       sync.Mutex
	lockMu   sync.Mutex
	checks   map[string]model.Check
	byToken  map[string]string
	pings    []model.Ping
	channels map[string]model.Channel
	alerts   []model.Alert
	nextPing int64
}

// New returns an empty store.
func New() *Mem {
	return &Mem{
		checks:   map[string]model.Check{},
		byToken:  map[string]string{},
		channels: map[string]model.Channel{},
	}
}

func (m *Mem) Ping(context.Context) error { return nil }

func (m *Mem) CreateCheck(_ context.Context, c model.Check) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.checks[c.ID]; ok {
		return store.ErrConflict
	}
	if _, ok := m.byToken[c.PingToken]; ok {
		return store.ErrConflict
	}
	m.checks[c.ID] = cloneCheck(c)
	m.byToken[c.PingToken] = c.ID
	return nil
}

func (m *Mem) UpdateCheck(_ context.Context, id, name string, period, grace int, now time.Time) (model.Check, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.checks[id]
	if !ok {
		return model.Check{}, store.ErrNotFound
	}
	c.Name = name
	c.PeriodSeconds = period
	c.GraceSeconds = grace
	c.UpdatedAt = now.UTC()
	m.checks[id] = c
	return cloneCheck(c), nil
}

func (m *Mem) GetCheck(_ context.Context, id string) (model.Check, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.checks[id]
	if !ok {
		return model.Check{}, store.ErrNotFound
	}
	return cloneCheck(c), nil
}

func (m *Mem) GetCheckByToken(_ context.Context, token string) (model.Check, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byToken[token]
	if !ok {
		return model.Check{}, store.ErrNotFound
	}
	return cloneCheck(m.checks[id]), nil
}

func (m *Mem) ListChecks(context.Context) ([]model.Check, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.Check, 0, len(m.checks))
	for _, c := range m.checks {
		out = append(out, cloneCheck(c))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].Name < out[j].Name
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (m *Mem) DeleteCheck(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.checks[id]
	if !ok {
		return store.ErrNotFound
	}
	delete(m.byToken, c.PingToken)
	delete(m.checks, id)
	m.pings = filter(m.pings, func(p model.Ping) bool { return p.CheckID != id })
	m.alerts = filterAlerts(m.alerts, func(a model.Alert) bool { return a.CheckID != id })
	return nil
}

func (m *Mem) ApplyPing(_ context.Context, token string, at time.Time, ip, ua string, channels []model.Channel) (model.PingResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byToken[token]
	if !ok {
		return model.PingResult{}, store.ErrNotFound
	}
	c := m.checks[id]
	at = at.UTC()
	m.nextPing++
	m.pings = append(m.pings, model.Ping{
		ID: m.nextPing, CheckID: c.ID, ReceivedAt: at, RemoteIP: ip, UserAgent: ua,
	})
	status, recovery := check.NextAfterPing(c)
	c.Status = status
	c.LastPingAt = &at
	c.UpdatedAt = at
	m.checks[id] = c
	if recovery {
		m.resolveDown(id, at)
		if err := m.appendAlerts(c.ID, model.AlertRecovered, at, channels); err != nil {
			return model.PingResult{}, err
		}
	}
	return model.PingResult{Check: cloneCheck(c), NeedsRecovery: recovery}, nil
}

func (m *Mem) Pause(_ context.Context, id string, now time.Time) (model.Check, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.checks[id]
	if !ok {
		return model.Check{}, store.ErrNotFound
	}
	c.Paused = true
	c.UpdatedAt = now.UTC()
	m.checks[id] = c
	return cloneCheck(c), nil
}

func (m *Mem) Resume(_ context.Context, id string, now time.Time) (model.Check, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.checks[id]
	if !ok {
		return model.Check{}, store.ErrNotFound
	}
	now = now.UTC()
	status, resolve := check.NextAfterResume(c, now)
	c.Paused = false
	c.Status = status
	c.UpdatedAt = now
	m.checks[id] = c
	if resolve {
		m.resolveDown(id, now)
	}
	return cloneCheck(c), nil
}

func (m *Mem) ClaimDue(_ context.Context, now time.Time) ([]model.Check, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now = now.UTC()
	var due []model.Check
	for id, c := range m.checks {
		if c.Paused || (c.Status != model.StatusNew && c.Status != model.StatusUp) {
			continue
		}
		if !check.IsOverdue(c, now) {
			continue
		}
		c.Status = model.StatusDown
		c.UpdatedAt = now
		m.checks[id] = c
		due = append(due, cloneCheck(c))
	}
	sort.Slice(due, func(i, j int) bool { return due[i].ID < due[j].ID })
	return due, nil
}

func (m *Mem) EnqueueDown(_ context.Context, now time.Time, channels []model.Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now = now.UTC()
	for id, c := range m.checks {
		if c.Paused || (c.Status != model.StatusNew && c.Status != model.StatusUp) {
			continue
		}
		if !check.IsOverdue(c, now) {
			continue
		}
		c.Status = model.StatusDown
		c.UpdatedAt = now
		m.checks[id] = c
		if err := m.appendAlerts(c.ID, model.AlertDown, now, channels); err != nil {
			return err
		}
	}
	return nil
}

func (m *Mem) ListPending(context.Context) ([]store.Pending, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []store.Pending{}
	for _, a := range m.alerts {
		if a.Status != model.AlertPending || a.ChannelID == nil {
			continue
		}
		c, ok := m.checks[a.CheckID]
		if !ok {
			continue
		}
		ch, ok := m.channels[*a.ChannelID]
		if !ok {
			continue
		}
		out = append(out, store.Pending{Alert: cloneAlert(a), Check: cloneCheck(c), Channel: cloneChannel(ch)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alert.SentAt.Before(out[j].Alert.SentAt) })
	return out, nil
}

func (m *Mem) SettleAlert(_ context.Context, id, status, errText string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.alerts {
		if m.alerts[i].ID != id || m.alerts[i].Status != model.AlertPending || m.alerts[i].ResolvedAt != nil {
			continue
		}
		m.alerts[i].Status = status
		m.alerts[i].Error = errText
		m.alerts[i].SentAt = at.UTC()
		break
	}
	return nil
}

func (m *Mem) appendAlerts(checkID, kind string, at time.Time, channels []model.Channel) error {
	rows, err := store.BuildAlertRows(checkID, kind, at, channels)
	if err != nil {
		return err
	}
	for _, a := range rows {
		m.alerts = append(m.alerts, cloneAlert(a))
	}
	return nil
}

func (m *Mem) CreateChannel(_ context.Context, c model.Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.channels[c.ID]; ok {
		return store.ErrConflict
	}
	m.channels[c.ID] = cloneChannel(c)
	return nil
}

func (m *Mem) UpdateChannel(_ context.Context, c model.Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.channels[c.ID]; !ok {
		return store.ErrNotFound
	}
	m.channels[c.ID] = cloneChannel(c)
	return nil
}

func (m *Mem) DeleteChannel(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.channels[id]; !ok {
		return store.ErrNotFound
	}
	delete(m.channels, id)
	for i := range m.alerts {
		if m.alerts[i].ChannelID != nil && *m.alerts[i].ChannelID == id {
			m.alerts[i].ChannelID = nil
		}
	}
	return nil
}

func (m *Mem) GetChannel(_ context.Context, id string) (model.Channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.channels[id]
	if !ok {
		return model.Channel{}, store.ErrNotFound
	}
	return cloneChannel(c), nil
}

func (m *Mem) ListChannels(context.Context) ([]model.Channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listChannels(false), nil
}

func (m *Mem) ListEnabledChannels(context.Context) ([]model.Channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listChannels(true), nil
}

func (m *Mem) listChannels(enabledOnly bool) []model.Channel {
	out := make([]model.Channel, 0, len(m.channels))
	for _, c := range m.channels {
		if enabledOnly && !c.Enabled {
			continue
		}
		out = append(out, cloneChannel(c))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (m *Mem) InsertAlert(_ context.Context, a model.Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, cloneAlert(a))
	return nil
}

func (m *Mem) ListAlerts(_ context.Context, checkID string, limit int) ([]model.Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.Alert
	for i := len(m.alerts) - 1; i >= 0; i-- {
		if m.alerts[i].CheckID == checkID {
			out = append(out, cloneAlert(m.alerts[i]))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SentAt.Equal(out[j].SentAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].SentAt.After(out[j].SentAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []model.Alert{}
	}
	return out, nil
}

func (m *Mem) ListPings(_ context.Context, checkID string, limit int) ([]model.Ping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.Ping
	for _, p := range m.pings {
		if p.CheckID == checkID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ReceivedAt.Equal(out[j].ReceivedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].ReceivedAt.After(out[j].ReceivedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []model.Ping{}
	}
	return out, nil
}

func (m *Mem) InsertPing(_ context.Context, p model.Ping) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextPing++
	p.ID = m.nextPing
	p.ReceivedAt = p.ReceivedAt.UTC()
	m.pings = append(m.pings, p)
	return nil
}

func (m *Mem) DeletePingsBefore(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	kept := m.pings[:0]
	for _, p := range m.pings {
		if p.ReceivedAt.Before(before) {
			n++
			continue
		}
		kept = append(kept, p)
	}
	m.pings = kept
	return n, nil
}

func (m *Mem) WithLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	if !m.lockMu.TryLock() {
		return false, nil
	}
	defer m.lockMu.Unlock()
	return true, fn(ctx)
}

func (m *Mem) RestoreDemoCheck(_ context.Context, id, status string, lastPing, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.checks[id]
	if !ok {
		return store.ErrNotFound
	}
	lastPing = lastPing.UTC()
	now = now.UTC()
	c.Status = status
	c.Paused = false
	c.LastPingAt = &lastPing
	c.UpdatedAt = now
	m.checks[id] = c
	var latest int = -1
	for i := range m.pings {
		if m.pings[i].CheckID != id {
			continue
		}
		if latest == -1 || m.pings[i].ReceivedAt.After(m.pings[latest].ReceivedAt) || (m.pings[i].ReceivedAt.Equal(m.pings[latest].ReceivedAt) && m.pings[i].ID > m.pings[latest].ID) {
			latest = i
		}
	}
	if latest >= 0 {
		m.pings[latest].ReceivedAt = lastPing
	}
	return nil
}

func (m *Mem) resolveDown(checkID string, at time.Time) {
	for i := range m.alerts {
		if m.alerts[i].CheckID == checkID && m.alerts[i].Kind == model.AlertDown && m.alerts[i].ResolvedAt == nil {
			t := at
			m.alerts[i].ResolvedAt = &t
		}
	}
}

func cloneCheck(c model.Check) model.Check {
	if c.LastPingAt != nil {
		t := c.LastPingAt.UTC()
		c.LastPingAt = &t
	}
	return c
}

func cloneChannel(c model.Channel) model.Channel {
	if c.Config != nil {
		c.Config = append(json.RawMessage(nil), c.Config...)
	}
	return c
}

func cloneAlert(a model.Alert) model.Alert {
	if a.ChannelID != nil {
		id := *a.ChannelID
		a.ChannelID = &id
	}
	if a.ResolvedAt != nil {
		t := a.ResolvedAt.UTC()
		a.ResolvedAt = &t
	}
	return a
}

func filter[T any](in []T, keep func(T) bool) []T {
	out := in[:0]
	for _, v := range in {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

func filterAlerts(in []model.Alert, keep func(model.Alert) bool) []model.Alert {
	return filter(in, keep)
}
