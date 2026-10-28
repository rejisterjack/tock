#!/bin/sh
# One-off task. Ping only after the work finishes.
# A failure before the curl leaves the check silent, which is the signal Tock watches.
set -eu

/opt/jobs/rebuild-report
curl -fsS "https://tock.example.com/ping/YOUR_TOKEN"
