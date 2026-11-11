# Security

Tock stores job names, schedules, and notification settings. Ping URLs are secrets: anyone who has one can record a success for that check.

Replace the example admin password, session secret, and API token before pointing `TOCK_BASE_URL` at a public host. Tock refuses to start when those example values are still set. Dashboard sign-in slows down after repeated failures. Slack and webhook deliveries do not follow redirects, so an `Authorization` header is not sent to a second host.

The container healthcheck uses `/readyz`. A process whose database is down is not reported healthy.

Report a vulnerability by email to rejisterjack@gmail.com. Please do not open a public GitHub issue for an undisclosed security problem.

Include what you found, how to reproduce it, and the version or commit you tested. You should hear back within a few days.
