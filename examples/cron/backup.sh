#!/bin/sh
# crontab example. The ping runs only when the backup succeeds.
# 15 2 * * * /opt/jobs/backup.sh
set -eu

dump="/var/backups/app-$(date -u +%Y%m%d).sql.gz"
pg_dump "$DATABASE_URL" | gzip > "$dump"

curl -fsS "https://tock.example.com/ping/YOUR_TOKEN"
