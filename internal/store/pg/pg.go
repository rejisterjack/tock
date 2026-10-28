// Package pg is the Postgres Repo.
package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rejisterjack/tock/internal/model"
	"github.com/rejisterjack/tock/internal/store"
)

const lockKey int64 = 748217001

// PG is a Postgres-backed store.
type PG struct {
	pool *pgxpool.Pool
}

// New wraps a pool.
func New(pool *pgxpool.Pool) *PG {
	return &PG{pool: pool}
}

// Close releases the pool.
func (p *PG) Close() { p.pool.Close() }

func (p *PG) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *PG) CreateCheck(ctx context.Context, c model.Check) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO checks (
			id, name, ping_token, period_seconds, grace_seconds, status,
			last_ping_at, paused, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, c.ID, c.Name, c.PingToken, c.PeriodSeconds, c.GraceSeconds, c.Status,
		c.LastPingAt, c.Paused, c.CreatedAt.UTC(), c.UpdatedAt.UTC())
	return mapErr(err)
}

func (p *PG) UpdateCheck(ctx context.Context, id, name string, period, grace int, now time.Time) (model.Check, error) {
	row := p.pool.QueryRow(ctx, `
		UPDATE checks
		SET name = $2, period_seconds = $3, grace_seconds = $4, updated_at = $5
		WHERE id = $1
		RETURNING id, name, ping_token, period_seconds, grace_seconds, status, last_ping_at, paused, created_at, updated_at
	`, id, name, period, grace, now.UTC())
	return scanCheck(row)
}

func (p *PG) GetCheck(ctx context.Context, id string) (model.Check, error) {
	row := p.pool.QueryRow(ctx, checkSelect+` WHERE id = $1`, id)
	return scanCheck(row)
}

func (p *PG) GetCheckByToken(ctx context.Context, token string) (model.Check, error) {
	row := p.pool.QueryRow(ctx, checkSelect+` WHERE ping_token = $1`, token)
	return scanCheck(row)
}

func (p *PG) ListChecks(ctx context.Context) ([]model.Check, error) {
	rows, err := p.pool.Query(ctx, checkSelect+` ORDER BY created_at DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectChecks(rows)
}

func (p *PG) DeleteCheck(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM checks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (p *PG) ApplyPing(ctx context.Context, token string, at time.Time, ip, ua string, channels []model.Channel) (model.PingResult, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return model.PingResult{}, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, checkSelect+` WHERE ping_token = $1 FOR UPDATE`, token)
	c, err := scanCheck(row)
	if err != nil {
		return model.PingResult{}, err
	}
	at = at.UTC()
	if _, err := tx.Exec(ctx, `
		INSERT INTO pings (check_id, received_at, remote_ip, user_agent)
		VALUES ($1, $2, $3, $4)
	`, c.ID, at, ip, ua); err != nil {
		return model.PingResult{}, err
	}

	needsRecovery := !c.Paused && c.Status == model.StatusDown
	newStatus := c.Status
	if !c.Paused {
		newStatus = model.StatusUp
	}
	updated := tx.QueryRow(ctx, `
		UPDATE checks
		SET status = $2, last_ping_at = $3, updated_at = $3
		WHERE id = $1
		RETURNING id, name, ping_token, period_seconds, grace_seconds, status, last_ping_at, paused, created_at, updated_at
	`, c.ID, newStatus, at)
	c, err = scanCheck(updated)
	if err != nil {
		return model.PingResult{}, err
	}
	if needsRecovery {
		if _, err := tx.Exec(ctx, `
			UPDATE alerts SET resolved_at = $2
			WHERE check_id = $1 AND kind = 'down' AND resolved_at IS NULL
		`, c.ID, at); err != nil {
			return model.PingResult{}, err
		}
		if err := insertAlertRows(ctx, tx, c.ID, model.AlertRecovered, at, channels); err != nil {
			return model.PingResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return model.PingResult{}, err
	}
	return model.PingResult{Check: c, NeedsRecovery: needsRecovery}, nil
}

func (p *PG) Pause(ctx context.Context, id string, now time.Time) (model.Check, error) {
	row := p.pool.QueryRow(ctx, `
		UPDATE checks SET paused = TRUE, updated_at = $2 WHERE id = $1
		RETURNING id, name, ping_token, period_seconds, grace_seconds, status, last_ping_at, paused, created_at, updated_at
	`, id, now.UTC())
	return scanCheck(row)
}

func (p *PG) Resume(ctx context.Context, id string, now time.Time) (model.Check, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return model.Check{}, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		UPDATE checks SET
			paused = FALSE,
			status = CASE
				WHEN COALESCE(last_ping_at, created_at) + (period_seconds + grace_seconds) * interval '1 second' >= $2
					AND status = 'down' AND last_ping_at IS NOT NULL THEN 'up'
				WHEN COALESCE(last_ping_at, created_at) + (period_seconds + grace_seconds) * interval '1 second' >= $2
					AND status = 'down' AND last_ping_at IS NULL THEN 'new'
				ELSE status
			END,
			updated_at = $2
		WHERE id = $1
		RETURNING id, name, ping_token, period_seconds, grace_seconds, status, last_ping_at, paused, created_at, updated_at
	`, id, now.UTC())
	c, err := scanCheck(row)
	if err != nil {
		return model.Check{}, err
	}
	if c.Status != model.StatusDown {
		if _, err := tx.Exec(ctx, `
			UPDATE alerts SET resolved_at = $2
			WHERE check_id = $1 AND kind = 'down' AND resolved_at IS NULL
		`, id, now.UTC()); err != nil {
			return model.Check{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Check{}, err
	}
	return c, nil
}

func (p *PG) ClaimDue(ctx context.Context, now time.Time) ([]model.Check, error) {
	rows, err := p.pool.Query(ctx, `
		UPDATE checks
		SET status = 'down', updated_at = $1
		WHERE NOT paused
		  AND status IN ('new', 'up')
		  AND COALESCE(last_ping_at, created_at) + (period_seconds + grace_seconds) * interval '1 second' < $1
		RETURNING id, name, ping_token, period_seconds, grace_seconds, status, last_ping_at, paused, created_at, updated_at
	`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectChecks(rows)
}

func (p *PG) EnqueueDown(ctx context.Context, now time.Time, channels []model.Channel) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		UPDATE checks
		SET status = 'down', updated_at = $1
		WHERE NOT paused
		  AND status IN ('new', 'up')
		  AND COALESCE(last_ping_at, created_at) + (period_seconds + grace_seconds) * interval '1 second' < $1
		RETURNING id, name, ping_token, period_seconds, grace_seconds, status, last_ping_at, paused, created_at, updated_at
	`, now.UTC())
	if err != nil {
		return err
	}
	due, err := collectChecks(rows)
	rows.Close()
	if err != nil {
		return err
	}
	for _, c := range due {
		if err := insertAlertRows(ctx, tx, c.ID, model.AlertDown, now.UTC(), channels); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *PG) ListPending(ctx context.Context) ([]store.Pending, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT a.id, a.check_id, a.kind, a.channel_id, a.channel_name, a.sent_at, a.status, a.error, a.resolved_at,
		       c.id, c.name, c.ping_token, c.period_seconds, c.grace_seconds, c.status, c.last_ping_at, c.paused, c.created_at, c.updated_at,
		       ch.id, ch.kind, ch.name, ch.config, ch.enabled, ch.created_at, ch.updated_at
		FROM alerts a
		JOIN checks c ON c.id = a.check_id
		JOIN channels ch ON ch.id = a.channel_id
		WHERE a.status = 'pending' AND a.resolved_at IS NULL
		ORDER BY a.sent_at ASC, a.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.Pending{}
	for rows.Next() {
		var item store.Pending
		var cfg []byte
		if err := rows.Scan(
			&item.Alert.ID, &item.Alert.CheckID, &item.Alert.Kind, &item.Alert.ChannelID, &item.Alert.ChannelName, &item.Alert.SentAt, &item.Alert.Status, &item.Alert.Error, &item.Alert.ResolvedAt,
			&item.Check.ID, &item.Check.Name, &item.Check.PingToken, &item.Check.PeriodSeconds, &item.Check.GraceSeconds, &item.Check.Status, &item.Check.LastPingAt, &item.Check.Paused, &item.Check.CreatedAt, &item.Check.UpdatedAt,
			&item.Channel.ID, &item.Channel.Kind, &item.Channel.Name, &cfg, &item.Channel.Enabled, &item.Channel.CreatedAt, &item.Channel.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Channel.Config = cfg
		out = append(out, item)
	}
	return out, rows.Err()
}

func (p *PG) SettleAlert(ctx context.Context, id, status, errText string, at time.Time) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE alerts SET status = $2, error = $3, sent_at = $4
		WHERE id = $1 AND status = 'pending' AND resolved_at IS NULL
	`, id, status, errText, at.UTC())
	return err
}

func insertAlertRows(ctx context.Context, tx pgx.Tx, checkID, kind string, at time.Time, channels []model.Channel) error {
	rows, err := store.BuildAlertRows(checkID, kind, at, channels)
	if err != nil {
		return err
	}
	for _, a := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO alerts (id, check_id, kind, channel_id, channel_name, sent_at, status, error, resolved_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, a.ID, a.CheckID, a.Kind, a.ChannelID, a.ChannelName, a.SentAt.UTC(), a.Status, a.Error, a.ResolvedAt); err != nil {
			return err
		}
	}
	return nil
}

func (p *PG) CreateChannel(ctx context.Context, c model.Channel) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO channels (id, kind, name, config, enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, c.ID, c.Kind, c.Name, []byte(c.Config), c.Enabled, c.CreatedAt.UTC(), c.UpdatedAt.UTC())
	return mapErr(err)
}

func (p *PG) UpdateChannel(ctx context.Context, c model.Channel) error {
	tag, err := p.pool.Exec(ctx, `
		UPDATE channels
		SET name = $2, config = $3, enabled = $4, updated_at = $5
		WHERE id = $1
	`, c.ID, c.Name, []byte(c.Config), c.Enabled, c.UpdatedAt.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (p *PG) DeleteChannel(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (p *PG) GetChannel(ctx context.Context, id string) (model.Channel, error) {
	row := p.pool.QueryRow(ctx, channelSelect+` WHERE id = $1`, id)
	return scanChannel(row)
}

func (p *PG) ListChannels(ctx context.Context) ([]model.Channel, error) {
	return p.queryChannels(ctx, channelSelect+` ORDER BY created_at ASC`)
}

func (p *PG) ListEnabledChannels(ctx context.Context) ([]model.Channel, error) {
	return p.queryChannels(ctx, channelSelect+` WHERE enabled ORDER BY created_at ASC`)
}

func (p *PG) InsertAlert(ctx context.Context, a model.Alert) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO alerts (id, check_id, kind, channel_id, channel_name, sent_at, status, error, resolved_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, a.ID, a.CheckID, a.Kind, a.ChannelID, a.ChannelName, a.SentAt.UTC(), a.Status, a.Error, a.ResolvedAt)
	return mapErr(err)
}

func (p *PG) ListAlerts(ctx context.Context, checkID string, limit int) ([]model.Alert, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, check_id, kind, channel_id, channel_name, sent_at, status, error, resolved_at
		FROM alerts
		WHERE check_id = $1
		ORDER BY sent_at DESC, id DESC
		LIMIT $2
	`, checkID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Alert{}
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (p *PG) ListPings(ctx context.Context, checkID string, limit int) ([]model.Ping, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, check_id, received_at, remote_ip, user_agent
		FROM pings
		WHERE check_id = $1
		ORDER BY received_at DESC, id DESC
		LIMIT $2
	`, checkID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Ping{}
	for rows.Next() {
		var pong model.Ping
		if err := rows.Scan(&pong.ID, &pong.CheckID, &pong.ReceivedAt, &pong.RemoteIP, &pong.UserAgent); err != nil {
			return nil, err
		}
		out = append(out, pong)
	}
	return out, rows.Err()
}

func (p *PG) InsertPing(ctx context.Context, pong model.Ping) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO pings (check_id, received_at, remote_ip, user_agent)
		VALUES ($1,$2,$3,$4)
	`, pong.CheckID, pong.ReceivedAt.UTC(), pong.RemoteIP, pong.UserAgent)
	return err
}

func (p *PG) DeletePingsBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM pings WHERE received_at < $1`, before.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (p *PG) WithLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey).Scan(&locked); err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey)
	}()
	return true, fn(ctx)
}

func (p *PG) RestoreDemoCheck(ctx context.Context, id, status string, lastPing, now time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE checks
		SET status = $2, paused = FALSE, last_ping_at = $3, updated_at = $4
		WHERE id = $1
	`, id, status, lastPing.UTC(), now.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		UPDATE pings SET received_at = $2
		WHERE id = (
			SELECT id FROM pings WHERE check_id = $1
			ORDER BY received_at DESC, id DESC
			LIMIT 1
		)
	`, id, lastPing.UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *PG) queryChannels(ctx context.Context, q string) ([]model.Channel, error) {
	rows, err := p.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Channel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const checkSelect = `
	SELECT id, name, ping_token, period_seconds, grace_seconds, status, last_ping_at, paused, created_at, updated_at
	FROM checks`

const channelSelect = `
	SELECT id, kind, name, config, enabled, created_at, updated_at
	FROM channels`

type scannable interface {
	Scan(dest ...any) error
}

func scanCheck(row scannable) (model.Check, error) {
	var c model.Check
	err := row.Scan(&c.ID, &c.Name, &c.PingToken, &c.PeriodSeconds, &c.GraceSeconds, &c.Status, &c.LastPingAt, &c.Paused, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Check{}, store.ErrNotFound
	}
	return c, err
}

func collectChecks(rows pgx.Rows) ([]model.Check, error) {
	out := []model.Check{}
	for rows.Next() {
		c, err := scanCheck(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanChannel(row scannable) (model.Channel, error) {
	var c model.Channel
	var cfg []byte
	err := row.Scan(&c.ID, &c.Kind, &c.Name, &cfg, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Channel{}, store.ErrNotFound
	}
	if err != nil {
		return model.Channel{}, err
	}
	c.Config = cfg
	return c, nil
}

func scanAlert(row scannable) (model.Alert, error) {
	var a model.Alert
	err := row.Scan(&a.ID, &a.CheckID, &a.Kind, &a.ChannelID, &a.ChannelName, &a.SentAt, &a.Status, &a.Error, &a.ResolvedAt)
	return a, err
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w", store.ErrConflict)
	}
	return err
}
