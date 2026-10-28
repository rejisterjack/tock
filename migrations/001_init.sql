-- +goose Up
CREATE TABLE checks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    ping_token TEXT NOT NULL UNIQUE,
    period_seconds INTEGER NOT NULL CHECK (period_seconds >= 60 AND period_seconds <= 31536000),
    grace_seconds INTEGER NOT NULL CHECK (grace_seconds >= 0 AND grace_seconds <= 31536000),
    status TEXT NOT NULL CHECK (status IN ('new', 'up', 'down')),
    last_ping_at TIMESTAMPTZ,
    paused BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX checks_due_idx ON checks (paused, status);

CREATE TABLE pings (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    check_id TEXT NOT NULL REFERENCES checks (id) ON DELETE CASCADE,
    received_at TIMESTAMPTZ NOT NULL,
    remote_ip TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT ''
);

CREATE INDEX pings_check_received_idx ON pings (check_id, received_at DESC);

CREATE TABLE channels (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('email', 'slack', 'webhook')),
    name TEXT NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE alerts (
    id TEXT PRIMARY KEY,
    check_id TEXT NOT NULL REFERENCES checks (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('down', 'recovered')),
    channel_id TEXT REFERENCES channels (id) ON DELETE SET NULL,
    channel_name TEXT NOT NULL DEFAULT '',
    sent_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('sent', 'failed')),
    error TEXT NOT NULL DEFAULT '',
    resolved_at TIMESTAMPTZ
);

-- One unresolved down delivery per check per channel. Idempotency for a
-- check with no channels is enforced by the status transition, not this index,
-- because channel deletion nulls channel_id and NULLs are distinct in unique indexes.
CREATE UNIQUE INDEX alerts_one_open_down_per_channel
    ON alerts (check_id, channel_id)
    WHERE kind = 'down' AND resolved_at IS NULL AND channel_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS channels;
DROP TABLE IF EXISTS pings;
DROP TABLE IF EXISTS checks;
