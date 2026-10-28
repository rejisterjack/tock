# Tock

Your cron jobs should prove they ran. Tock alerts you when the proof stops arriving.

Tock is a self-hosted dead-man's switch. Each job pings a unique URL when it succeeds. If that ping is overdue, Tock notifies you once by email, Slack, or webhook, and once more when the job recovers.

[Healthchecks.io](https://healthchecks.io) is a good hosted version of this. Use it if you do not need to keep job metadata on your own machine. Tock is the small self-hosted alternative: one Compose file, Postgres, no account.

## Quick start

```sh
git clone https://github.com/rejisterjack/tock.git
cd tock
cp .env.example .env
```

Change `TOCK_ADMIN_PASSWORD`, `TOCK_SESSION_SECRET`, and `TOCK_API_TOKEN` in `.env`, then:

```sh
docker compose up -d
```

Open [http://localhost:8080](http://localhost:8080) and sign in with `TOCK_ADMIN_PASSWORD`.

## Create a check

1. Create a check with a name, an expected interval, and a grace period.
2. Copy the ping URL Tock generates.
3. Append one line to the job.

```sh
curl -fsS https://tock.example.com/ping/TOKEN
```

GET and POST both count as success. The grace period is extra time after the interval before Tock marks the check down, so a slightly late job does not page you.

A check that has never pinged stays **new**. If the first ping misses the interval plus grace, measured from creation, Tock marks it down and alerts.

Pause a check during maintenance. Pings are still recorded, and alerts stay off until you resume. Resume judges the schedule from the last ping. A ping that arrived while the check was paused brings it back to up without a recovery notice. A check that is already down, and still overdue, stays down so Tock does not send a second alert. Pause does not freeze the clock.

## Notifications

Every enabled channel receives each down alert and each recovery. A down alert is sent once. The next success ping sends one recovery. The ping token is not included in the message.

### Email

SMTP is process configuration, because it belongs to your mail server. Recipients are a channel in the dashboard.

```
TOCK_SMTP_HOST=smtp.example.com
TOCK_SMTP_PORT=587
TOCK_SMTP_USER=
TOCK_SMTP_PASSWORD=
TOCK_SMTP_FROM=tock@example.com
```

Port 465 uses implicit TLS. Any other port tries STARTTLS and still sends if the server does not offer it. An empty `TOCK_SMTP_HOST` does not stop Tock from starting. An email channel then records a failed alert, which the dashboard shows.

A down alert row is written in the same database step that marks the check down. If the process dies before the message goes out, the row stays `pending` and the next tick sends it. A delivery that fails is stored as `failed` and is not retried.

Verified on 29 Oct 2026 with Mailpit (`docker compose --profile smtp up`). Check `mailpit-proof` had a 60 second period and no grace. Mailpit received `Tock: mailpit-proof is down`, then `Tock: mailpit-proof recovered` after the ping URL was called. Each enabled email channel got one message per event. Slack's JSON shape is covered by `TestSlackAndWebhook` in `internal/notify/notify_test.go`.

To try mail locally:

```sh
docker compose --profile smtp up -d
```

In `.env` set `TOCK_SMTP_HOST=mailpit`, `TOCK_SMTP_PORT=1025`, and `TOCK_SMTP_FROM=tock@localhost`. Recreate the app and open [http://localhost:8025](http://localhost:8025). Add an email channel whose recipient is any address. Mailpit accepts it.

### Slack

Add a Slack channel and paste an [incoming webhook](https://api.slack.com/messaging/webhooks) URL. Tock posts a text message.

### Webhook

Add a webhook channel. Tock POSTs JSON:

```json
{
  "event": "down",
  "check": {
    "id": "…",
    "name": "nightly-backup",
    "status": "down",
    "period_seconds": 86400,
    "grace_seconds": 3600,
    "paused": false
  },
  "last_ping_at": "2026-10-28T10:00:00Z",
  "sent_at": "2026-10-28T12:00:00Z"
}
```

`event` is `down` or `recovered`. Optional headers are one `Name: value` per line in the form, or a `headers` object in the API. A failed delivery is stored on the alert and is not retried on the next tick.

To watch a webhook from Compose, run `nc -l 9999` on the host and set the channel URL to `http://host.docker.internal:9999`.

## Examples

- [Cron backup](examples/cron/backup.sh) — ping only after `pg_dump` succeeds.
- [Worker loop](examples/worker/loop.sh) — ping at the end of each pass. If the process dies, the pings stop.
- [One-off task](examples/one-off/task.sh) — ping after the script finishes.

## API

```sh
curl -s -X POST http://localhost:8080/api/v1/checks \
  -H "Authorization: Bearer $TOCK_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"nightly-backup","period_seconds":86400,"grace_seconds":3600}'
```

The response includes `ping_url` and `curl`.

| Method | Path |
| --- | --- |
| GET, POST | `/api/v1/checks` |
| GET, PATCH, DELETE | `/api/v1/checks/{id}` |
| POST | `/api/v1/checks/{id}/pause` and `/resume` |
| GET | `/api/v1/checks/{id}/pings` and `/alerts` |
| GET, POST | `/api/v1/channels` |
| PATCH, DELETE | `/api/v1/channels/{id}` |

PATCH accepts any of `name`, `period_seconds`, and `grace_seconds`. Omitted channel fields stay as they are. Channel `kind` is `email`, `slack`, or `webhook`.

```json
{"kind":"email","name":"ops","enabled":true,"config":{"to":["ops@example.com"]}}
{"kind":"slack","name":"ops","config":{"webhook_url":"https://hooks.slack.com/services/…"}}
{"kind":"webhook","name":"ops","config":{"url":"https://example.com/tock","headers":{"Authorization":"Bearer …"}}}
```

`GET /healthz` and `GET /readyz` do not require a session. `/readyz` checks Postgres.

## Configuration

Required:

- `DATABASE_URL`
- `TOCK_BASE_URL` — public origin. A trailing slash is stripped.
- `TOCK_ADMIN_PASSWORD` — at least 8 characters
- `TOCK_SESSION_SECRET` — at least 16 characters. Signs the dashboard cookie.
- `TOCK_API_TOKEN` — at least 16 characters

Optional:

- `TOCK_ADDR` — default `:8080`
- `TOCK_SMTP_HOST`, `TOCK_SMTP_PORT` (default 587 when the host is set), `TOCK_SMTP_USER`, `TOCK_SMTP_PASSWORD`, `TOCK_SMTP_FROM`
- `TOCK_PING_RETENTION_DAYS` — default 90, then old ping rows are deleted
- `TOCK_DEMO` — `true` or `false`
- `TOCK_TRUST_PROXY` — `true` only when a reverse proxy sets `X-Forwarded-For`. Otherwise ping history stores the socket address.

Sessions last 7 days. The cookie is `Secure` when `TOCK_BASE_URL` is `https`.

Run one app container. A second replica takes a Postgres advisory lock around each tick so a check is not alerted twice.

## TLS

Terminate TLS at a reverse proxy. Caddy:

```
tock.example.com {
  reverse_proxy localhost:8080
}
```

Set `TOCK_BASE_URL=https://tock.example.com`.

## Demo

`TOCK_DEMO=true` seeds three synthetic jobs — one up, one inside its grace window, and one down — and keeps those clocks aligned. Outbound email, Slack, and webhooks are refused. The dashboard can be read without signing in. Creating or editing checks and channels is rejected. Ping URLs answer `OK` and do not change the jobs.

The public demo is [tock.rejisterjack.com](https://tock.rejisterjack.com). That host should run with `TOCK_DEMO=true` only.

### Railway

1. Create a Railway project and add Postgres.
2. Deploy this repo with the Dockerfile. The Go process serves the dashboard. Do not put the dashboard on Vercel.
3. Set `DATABASE_URL` from the Postgres plugin, `TOCK_BASE_URL=https://tock.rejisterjack.com`, a password, a session secret, an API token, and `TOCK_DEMO=true`. Leave SMTP unset.
4. Attach the domain `tock.rejisterjack.com`.

`TOCK_PING_RETENTION_DAYS=2` is enough for the demo, because the simulator rewrites the sample pings.

## Backup

Postgres holds the checks, ping tokens, and channel settings. Dump it with `pg_dump`. Treat the dump as a secret: the ping URLs are the only authentication on the ping endpoint.

## Development

Requires Go 1.27.3.

```sh
go test ./...
go vet ./...
DATABASE_URL=postgres://tock:tock@localhost:5432/tock?sslmode=disable go test -tags integration ./...
```

`go test ./...` does not need a database. The `integration` tag runs the same down, recovery, and retention transitions against Postgres.

Tagged releases publish `ghcr.io/rejisterjack/tock`. The Compose file builds from this directory, so a clone does not need the registry.

## What this is not

Tock is not an APM, a log store, or a tracer. A schedule is an interval plus a grace period. Cron expressions, explicit start and fail signals, per-check channel routing, and multiple user accounts are out of this release.

## License

[MIT](LICENSE). The dashboard vendors [Alpine.js](https://alpinejs.dev/), which is also MIT.
