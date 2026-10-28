#!/bin/sh
# Long-running worker. Ping after each successful pass.
# If the process dies, the pings stop and Tock alerts after the period plus grace.
set -eu

while true; do
  /opt/jobs/process-queue
  curl -fsS "https://tock.example.com/ping/YOUR_TOKEN"
  sleep 300
done
