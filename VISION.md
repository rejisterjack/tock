# Tock — Cron Job Monitor

> Your cron jobs should prove they ran. Tock alerts you when the proof stops arriving.

**Home:** [tock.rejisterjack.com](https://tock.rejisterjack.com)

## One-line idea

A lightweight dead-man’s-switch monitor with a dashboard: each job pings when it succeeds, and Tock alerts you if the ping is missing—self-hosted with Docker Compose.

## The problem

Cron jobs and scheduled tasks fail silently. A backup or digest can stop running for weeks before anyone notices. Hosted services like Healthchecks.io solve this well but charge for teams and store your job metadata on their infrastructure. Tock is a simple, free, self-hosted alternative: ping on success, alert on absence.

## What Tock does

- Unique ping URL per monitored job
- Jobs signal success with a simple HTTP request (e.g. `curl`) after they finish
- Alerts via email, Slack, or outbound webhook when a ping is overdue
- Dashboard: jobs, last ping, status, alert history
- Grace periods to reduce false alarms when jobs run slightly late

## Who it's for

- Anyone running production cron, backups, reports, or cleanup jobs
- Operators who need proof of execution and fast failure notification
- Small teams who want self-hosting without a recurring SaaS fee

## How you use it

1. Start Tock with Docker Compose (includes Postgres)
2. Create a check in the dashboard: name, expected interval, grace period
3. Copy the ping URL Tock generates
4. Append one line to your job, e.g. `curl -s https://your-tock/ping/<id>`
5. If pings stop, Tock notifies your configured channel

## How it runs

- Docker Compose bundles the Go receiver, dashboard, and Postgres
- `docker compose up -d` is the primary install path
- No required cloud dependency; runs on your machine or server
- Optional public demo may use synthetic jobs only

## Tech stack

- Go (ping receiver, alert scheduler, API)
- Postgres (checks, pings, alert history)
- Lightweight dashboard (HTML + Alpine.js)
- SMTP and Slack incoming webhooks for notifications

## Success criteria

- Single Compose file starts the full stack
- Examples for cron, worker, and one-off task patterns
- Email and Slack alerts verified in docs
- Dashboard shows status, last ping, and history
- README with integration snippet and demo
- Open source at [github.com/rejisterjack/tock](https://github.com/rejisterjack/tock)

## Limitations & non-goals

- Category is mature; differentiation is simplicity and self-hosting
- Healthchecks.io hosted tier is easier if you do not need self-hosting—say so in the README
- Reliable email alerts depend on proper SMTP setup (operator responsibility)
- Not full APM, log aggregation, or distributed tracing

## Deployment

| Mode | Notes |
|------|--------|
| Self-hosted | Docker Compose (recommended) |
| Demo | Optional Railway instance with fake jobs |

The dashboard is served by the Go process; Vercel is not part of the core architecture.
