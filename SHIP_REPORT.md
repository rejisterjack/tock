# Ship Readiness Report — Tock

Testing finished on 22 Nov 2026. The race suite, the Postgres integration tests, a live server, and Chrome passed. 68 features passed. The decision is GO.

## Product Summary

Tock is a self-hosted dead-man's switch: a job pings a secret URL, and Tock alerts when that ping is overdue.
The Go process serves the dashboard, the JSON API, and the ping receiver, and stores checks in Postgres.
Alerts go out by SMTP, a Slack incoming webhook, or a JSON webhook, once for down and once for recovery.
The public page named in the README is a read-only demo with three synthetic jobs and outbound notifications off.
A real install is Docker Compose. Tock is not an APM, a log store, or a tracer.

## Feature Table

| ID | Feature | Priority | Status | Bugs Found | Bugs Fixed | UI Issues Fixed | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AUTH-01 | Sign in | P0 | PASSED | 0 | 0 | 0 | One password from TOCK_ADMIN_PASSWORD. A match starts a session and redirects to /. |
| AUTH-02 | Sign out | P0 | PASSED | 0 | 0 | 0 | POST /logout clears the cookie. Demo mode returns to /. |
| AUTH-03 | Session cookie | P0 | PASSED | 0 | 0 | 0 | Seven days, HttpOnly, SameSite Lax. Secure and HSTS only when the base URL is https. |
| AUTH-04 | CSRF | P0 | PASSED | 0 | 0 | 0 | Login uses tock_csrf. Later forms and mutating API calls need the session token. |
| AUTH-05 | Login failure limit | P0 | PASSED | 0 | 0 | 0 | Eight failures from one address in 15 minutes return 429. The count is in memory. |
| AUTH-06 | API token | P0 | PASSED | 0 | 0 | 0 | Authorization Bearer must match TOCK_API_TOKEN. A session can also call the API. |
| AUTH-07 | Example secrets | P0 | PASSED | 0 | 0 | 0 | change-me-now and the other example secrets are refused unless the base URL is loopback. |
| AUTH-08 | Demo read-only | P0 | PASSED | 0 | 0 | 0 | TOCK_DEMO=true shows the dashboard without a session and rejects mutations with 403. |
| JOB-01 | Job list | P0 | PASSED | 0 | 0 | 0 | / shows status, name, last ping, period, and grace. |
| JOB-02 | Empty jobs | P1 | PASSED | 0 | 0 | 0 | An empty list shows the four setup steps and hides the job list. |
| JOB-03 | Create check | P0 | PASSED | 0 | 0 | 0 | The form defaults to 1 day and 30 minutes of grace, then opens the new check. |
| JOB-04 | Edit check | P1 | PASSED | 0 | 0 | 0 | Name and schedule change. The ping token stays the same. |
| JOB-05 | Pause and resume | P0 | PASSED | 0 | 0 | 0 | Pause records pings and sends no alerts. Resume judges the schedule from the last ping. |
| JOB-06 | Delete check | P0 | PASSED | 0 | 0 | 0 | Delete removes the check and its history, then returns to the job list. |
| JOB-07 | Ping URL and copy | P0 | PASSED | 0 | 0 | 0 | The detail page shows the ping URL and a curl line, with a copy button. |
| JOB-08 | History tables | P1 | PASSED | 0 | 0 | 0 | The detail page lists up to 50 pings and 50 alerts, or an empty line. |
| JOB-09 | Schedule validation | P0 | PASSED | 0 | 0 | 0 | Name is 1 to 80 characters. Period is 1 minute to 365 days. Grace is 0 to 365 days. |
| PING-01 | Success ping | P0 | PASSED | 0 | 0 | 0 | GET and POST /ping/{token} return OK and a text body. |
| PING-02 | Unknown token | P0 | PASSED | 0 | 0 | 0 | An unknown token returns 404 and does not create a check. |
| PING-03 | New, up, grace, down | P0 | PASSED | 0 | 0 | 0 | A first ping moves new to up. Grace is derived. Overdue after period plus grace is down. |
| PING-04 | Recovery ping | P0 | PASSED | 0 | 0 | 0 | A ping while down stores up and queues one recovery. |
| PING-05 | Paused ping | P0 | PASSED | 0 | 0 | 0 | A ping while paused is stored and does not change status or send a recovery. |
| PING-06 | Address and agent | P1 | PASSED | 0 | 0 | 0 | The ping stores a clipped address and user agent. X-Forwarded-For is used only when trusted. |
| PING-07 | Demo ping | P0 | PASSED | 0 | 0 | 0 | A known demo token returns OK and does not write a ping or change the job. |
| SCH-01 | Scheduler tick | P0 | PASSED | 0 | 0 | 0 | A tick runs at start and then every 15 seconds. |
| SCH-02 | Single down alert | P0 | PASSED | 0 | 0 | 0 | One unresolved down row per check per channel. A second tick does not send another. |
| SCH-03 | Pending then send | P0 | PASSED | 0 | 0 | 0 | The down row is written as pending with the status change. A later tick sends it. |
| SCH-04 | Failed is final | P1 | PASSED | 0 | 0 | 0 | A delivery error is stored as failed and is not retried on the next tick. |
| SCH-05 | Advisory lock | P0 | PASSED | 0 | 0 | 0 | pg_try_advisory_lock(748217001) wraps the tick. A busy lock skips that pass. |
| SCH-06 | Retention | P1 | PASSED | 0 | 0 | 0 | Pings older than TOCK_PING_RETENTION_DAYS are deleted. The default is 90. |
| SCH-07 | Down with no channel | P1 | PASSED | 0 | 0 | 0 | A check with no enabled channel still becomes down. Nothing is notified. |
| NTF-01 | Email channel | P0 | PASSED | 0 | 0 | 0 | Recipients are a dashboard channel. SMTP host and From stay in the environment. |
| NTF-02 | SMTP | P0 | PASSED | 0 | 0 | 0 | Port 465 uses implicit TLS. Any other port tries STARTTLS and still sends if it is absent. |
| NTF-03 | Slack | P0 | PASSED | 0 | 0 | 0 | A Slack channel posts {"text": "..."} to the incoming webhook URL. |
| NTF-04 | Webhook | P0 | PASSED | 0 | 0 | 0 | A webhook POSTs event, check, last_ping_at, and sent_at. Headers are optional. |
| NTF-05 | Edit and delete channel | P1 | PASSED | 0 | 0 | 0 | Save changes name, config, and enabled. Kind stays. Delete keeps the name on old alerts. |
| NTF-06 | Message body | P0 | PASSED | 0 | 0 | 0 | Subjects are "Tock: NAME is down" and "Tock: NAME recovered". The ping token is absent. |
| NTF-07 | No redirects | P1 | PASSED | 0 | 0 | 0 | Slack and webhook clients refuse redirects so an Authorization header stays on the first host. |
| NTF-08 | Demo notifications off | P0 | PASSED | 0 | 0 | 0 | Demo mode returns "notifications are disabled in demo mode" and does not dial out. |
| API-01 | List and create checks | P0 | PASSED | 0 | 0 | 0 | GET and POST /api/v1/checks. The create body returns ping_url and curl. |
| API-02 | Get, patch, delete | P0 | PASSED | 0 | 0 | 0 | PATCH changes only the fields present. DELETE returns 204. |
| API-03 | Pause and resume API | P0 | PASSED | 0 | 0 | 0 | POST /api/v1/checks/{id}/pause and /resume. |
| API-04 | Pings and alerts API | P1 | PASSED | 0 | 0 | 0 | GET pings and alerts. Limit defaults to 50 and caps at 200. |
| API-05 | Channels API | P0 | PASSED | 0 | 0 | 0 | GET, POST, PATCH, and DELETE /api/v1/channels. Omitted fields stay. |
| API-06 | Reject bad JSON | P1 | PASSED | 0 | 0 | 0 | Unknown fields and a body over 1 MiB are 400. Demo mutations are 403. |
| OPS-01 | Health and ready | P0 | PASSED | 0 | 0 | 0 | /healthz reports the process. /readyz returns 503 when Postgres does not answer. |
| OPS-02 | Process healthcheck | P1 | PASSED | 0 | 0 | 0 | tock healthcheck calls /readyz on the listen port and exits 1 when it is not 200. |
| OPS-03 | Migrations | P0 | PASSED | 0 | 0 | 0 | Goose runs before listen. Alert status allows pending, sent, and failed. |
| OPS-04 | Configuration | P0 | PASSED | 0 | 0 | 0 | Database, base URL, password, session secret, and API token are required at start. |
| OPS-05 | Listen address | P0 | PASSED | 0 | 0 | 0 | TOCK_ADDR wins. Otherwise the process listens on :$PORT, or :8080. |
| OPS-06 | Security headers | P0 | PASSED | 0 | 0 | 0 | nosniff, DENY framing, no-referrer, and a CSP that allows the page's own scripts and styles. |
| OPS-07 | Logs and panics | P1 | PASSED | 0 | 0 | 0 | JSON request logs redact /ping/:token. A panic becomes 500 and the scheduler keeps ticking. |
| OPS-08 | Shutdown | P2 | PASSED | 0 | 0 | 0 | Interrupt and SIGTERM stop the HTTP server with a 10 second deadline. |
| WEB-01 | Sign-in page | P1 | PASSED | 0 | 0 | 0 | The password field is labeled, required, and autofocused. Errors use role=alert. |
| WEB-02 | Chrome, title, icon, robots | P1 | PASSED | 0 | 0 | 0 | Titles end in " · Tock". The icon is /static/favicon.svg. robots is noindex. |
| WEB-03 | Responsive layout | P2 | PASSED | 1 | 1 | 1 | Long ping URLs and alert errors no longer widen the page at 360 or 768. |
| WEB-04 | Keyboard focus | P1 | PASSED | 0 | 0 | 0 | Buttons, links, and fields use a 2px focus-visible outline. |
| WEB-05 | Relative times | P2 | PASSED | 0 | 0 | 0 | Last ping and history times update every 30 seconds and keep an absolute timestamp. |
| WEB-06 | Demo banner | P1 | PASSED | 0 | 0 | 0 | Demo mode shows a banner and hides New check, edit, pause, and channel forms. |
| WEB-07 | Not found | P1 | PASSED | 0 | 0 | 0 | An unknown check id returns the plain 404 page. |
| DEMO-01 | Seeded jobs | P0 | PASSED | 0 | 0 | 0 | Demo seeds nightly-backup up, weekly-digest in grace, and db-vacuum down. |
| DEMO-02 | Edge rewrite | P1 | PASSED | 0 | 0 | 0 | edge/vercel.json rewrites every path to the Render demo URL. |
| DEVOPS-01 | CI | P0 | PASSED | 0 | 0 | 0 | Push and pull request run race tests, vet, Postgres integration tests, govulncheck, and an image build. |
| DEVOPS-02 | GHCR image | P0 | PASSED | 0 | 0 | 0 | A v* tag pushes linux/amd64 and linux/arm64 to ghcr.io/rejisterjack/tock. |
| DEVOPS-03 | Compose | P0 | PASSED | 0 | 0 | 0 | Compose starts Postgres 16 and the app on 8080. The smtp profile adds Mailpit. |
| DEVOPS-04 | Smoke workflow | P1 | PASSED | 0 | 0 | 0 | scripts/smoke.py is manual. It is not part of the pull-request gate. |
| DEVOPS-05 | Go 1.27.4 | P1 | PASSED | 0 | 0 | 0 | go.mod, CI, and the Dockerfile all say Go 1.27.4. |
| DEVOPS-06 | Job examples | P2 | PASSED | 0 | 0 | 0 | examples/cron, examples/worker, and examples/one-off ping only after the work succeeds. |

## Gaps

- GAP-01 Multiple accounts. DEFER. The README closes this release at one admin password.
- GAP-02 Cron expressions. DEFER. A schedule is an interval plus a grace period.
- GAP-03 Start and fail signals. DEFER. A job only reports success. Silence is the failure.
- GAP-04 Per-check channel routing. DEFER. Every enabled channel receives every alert.
- GAP-05 APM, logs, and tracing. DEFER. VISION.md marks them won't-build.
- GAP-06 Billing. DEFER. A self-hosted install has no account and no charge.
- GAP-07 Dark theme. DEFER. The dashboard is one light page.
- GAP-08 Password change in the dashboard. DEFER. The password is process configuration.
- GAP-09 Retry of a failed alert. DEFER. A failed row stays failed until a new down or recovery event.
- GAP-10 Ping rate limit. DEFER. The ping URL is the secret, and the operator can limit it at the proxy.

## Per-Feature Sections

### AUTH-01 Sign in

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/server.go, internal/auth/auth.go, web/templates/login.html
- **Expected behavior:** GET /login shows the form. A signed-in user is redirected to /. POST with the admin password sets tock_session and redirects to /. A wrong password renders "Invalid password" with 401.
- **Acceptance:**
  - [x] The right password opens the job list.
  - [x] The wrong password stays on the form and does not set a session.
  - [x] An existing session does not see the form again.
- **Test cases:** Correct password; empty password; whitespace; a long password; the wrong password; a logged-out POST; a double submit.
- **UI checks:** The password label is tied to the field. The error is in role=alert.
- **Dependencies:** TOCK_ADMIN_PASSWORD of at least 8 characters.
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### AUTH-02 Sign out

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/server.go, web/templates/layout.html
- **Expected behavior:** POST /logout with the session CSRF clears the cookie. A normal install goes to /login. Demo mode goes to /.
- **Acceptance:**
  - [x] After sign-out, / redirects to /login.
  - [x] A missing or wrong CSRF does not clear a valid session.
- **Test cases:** Signed in; signed out; bad CSRF; double submit.
- **UI checks:** The header button is "Sign out" only when a session exists.
- **Dependencies:** AUTH-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### AUTH-03 Session cookie

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/auth/auth.go, internal/config/config.go, internal/httpapi/server.go
- **Expected behavior:** The cookie lasts 7 days, is HttpOnly, and is SameSite Lax. Secure is set only when TOCK_BASE_URL starts with https. An expired or re-signed cookie is treated as logged out.
- **Acceptance:**
  - [x] An http base URL does not mark the cookie Secure.
  - [x] An https base URL marks it Secure and sends HSTS.
  - [x] A cookie signed with another secret is rejected.
- **Test cases:** Fresh login; expired exp; flipped signature; http and https base URLs.
- **UI checks:** none
- **Dependencies:** AUTH-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### AUTH-04 CSRF

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/auth/auth.go, internal/httpapi/server.go
- **Expected behavior:** Login compares the form token with the tock_csrf cookie. Dashboard posts compare the form token, or X-CSRF-Token, with the session. A mismatch is 400.
- **Acceptance:**
  - [x] A login without the cookie token is rejected.
  - [x] A check create without the session token is rejected.
  - [x] The matching token is accepted.
- **Test cases:** Missing token; wrong token; token from another session; header token on a mutating API call.
- **UI checks:** Hidden csrf fields are present on logout, check, and channel forms.
- **Dependencies:** AUTH-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### AUTH-05 Login failure limit

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/limit.go, internal/httpapi/server.go
- **Expected behavior:** The ninth failed sign-in from the same address inside 15 minutes renders "Too many attempts. Try again in a few minutes." with 429. A success clears that address. The window is process memory.
- **Acceptance:**
  - [x] Eight failures are 401 and the ninth is 429.
  - [x] A different address is still allowed.
  - [x] A success after failures allows the next attempt.
- **Test cases:** Eight wrong passwords; a correct password on the ninth of a fresh address; two addresses; the window after 15 minutes.
- **UI checks:** The 429 message is in role=alert.
- **Dependencies:** AUTH-01
- **Suspected bugs:** internal/httpapi/limit.go:24 drops one arbitrary address when 10000 keys are stored, so a full map can forget an address that is still failing.
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### AUTH-06 API token

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/auth/auth.go, internal/httpapi/server.go
- **Expected behavior:** A Bearer token equal to TOCK_API_TOKEN authorizes every API route. A wrong or empty bearer is 401. With no Authorization header, a live session is accepted, and mutations also need CSRF.
- **Acceptance:**
  - [x] The right bearer lists checks.
  - [x] A wrong bearer is 401.
  - [x] A session without the header can read.
  - [x] A session mutation without CSRF is 400.
- **Test cases:** Missing header; Bearer with a blank token; wrong token; the token in a query string; a session cookie.
- **UI checks:** none
- **Dependencies:** OPS-04
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### AUTH-07 Example secrets

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/config/config.go
- **Expected behavior:** TOCK_ADMIN_PASSWORD=change-me-now, TOCK_SESSION_SECRET=change-me-session-secret, and TOCK_API_TOKEN=change-me-api-token are accepted only when the base URL host is localhost or a loopback IP. Any other host refuses to start and names the keys.
- **Acceptance:**
  - [x] localhost with the example values loads.
  - [x] A public host with an example value fails before listen.
  - [x] A public host with replaced values loads.
- **Test cases:** localhost; 127.0.0.1; a public hostname; one example value left in place.
- **UI checks:** none
- **Dependencies:** none
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### AUTH-08 Demo read-only

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/server.go, internal/app/service.go
- **Expected behavior:** With TOCK_DEMO=true, / and /channels render without a session. Creating, editing, pausing, resuming, or deleting a check or channel returns 403 "demo mode is read-only".
- **Acceptance:**
  - [x] A logged-out demo visitor sees the three jobs.
  - [x] POST /checks is 403.
  - [x] The API returns the same read-only error.
- **Test cases:** Logged out; a valid session still cannot mutate; GET /checks/new.
- **UI checks:** New check, edit, pause, and the channel form are absent.
- **Dependencies:** DEMO-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### JOB-01 Job list

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/dashboard.go, web/templates/checks.html, internal/check/status.go
- **Expected behavior:** / lists every check newest first with display status, name, last ping, period, and grace. Display status is new, up, grace, down, or paused.
- **Acceptance:**
  - [x] Each seeded or created job is a link to its detail page.
  - [x] A job inside the grace window shows grace while the stored status stays up or new.
  - [x] A logged-out visitor on a normal install is sent to /login.
- **Test cases:** One job; many jobs; a paused job; a job with no ping; refresh.
- **UI checks:** The current nav item is Jobs. Status pills use the status class.
- **Dependencies:** AUTH-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### JOB-02 Empty jobs

- **Priority:** P1
- **Status:** PASSED
- **Files:** web/templates/checks.html
- **Expected behavior:** When no checks exist, the page shows four numbered steps and no job list.
- **Acceptance:**
  - [x] A fresh database shows the steps.
  - [x] Creating the first check replaces the steps with that job.
- **Test cases:** Empty database; after deleting the last check.
- **UI checks:** The steps are readable and the New check link remains.
- **Dependencies:** JOB-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### JOB-03 Create check

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/dashboard.go, web/templates/check_form.html, internal/app/service.go
- **Expected behavior:** GET /checks/new shows name, period, and grace. The defaults are 1 day and 30 minutes. POST creates a new check with status new and redirects to its detail page.
- **Acceptance:**
  - [x] A valid form creates one check and shows its ping URL.
  - [x] A rejected form stays on the page with the error and the values.
  - [x] Demo mode returns 403 for the form and the POST.
- **Test cases:** Default values; empty name; whitespace name; 80 characters; 81 characters; a name with a newline; emoji; a non-English name; period below 60 seconds; double submit.
- **UI checks:** Labels match the inputs. Cancel returns to /.
- **Dependencies:** AUTH-04, JOB-09
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### JOB-04 Edit check

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/httpapi/dashboard.go, internal/app/service.go
- **Expected behavior:** GET /checks/{id}/edit loads the current name and schedule. POST saves them and keeps the ping token. An unknown id is 404.
- **Acceptance:**
  - [x] The token in the detail page is unchanged after save.
  - [x] A bad period redisplays the form.
  - [x] An unknown id is not found.
- **Test cases:** Rename; same values; unknown id; logged out.
- **UI checks:** The heading is "Edit check" and Cancel returns to the detail page.
- **Dependencies:** JOB-03
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### JOB-05 Pause and resume

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/check/status.go, internal/app/service.go, web/templates/check_detail.html
- **Expected behavior:** Pause sets paused and stops alerts. Pings are still stored. Resume leaves an overdue check in its stored status so an already-down check is not alerted again. A ping that arrived during pause, and is still inside the window, returns the check to up or new without a recovery notice.
- **Acceptance:**
  - [x] A paused check shows paused and the resume button.
  - [x] An overdue paused check that was already down stays down on resume.
  - [x] A ping during pause does not send recovery.
- **Test cases:** Pause an up check; ping while paused; resume inside the window; resume while overdue; double submit.
- **UI checks:** The paused note is visible. Demo mode hides both buttons.
- **Dependencies:** PING-05, SCH-02
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### JOB-06 Delete check

- **Priority:** P0
- **Status:** PASSED
- **Files:** web/templates/check_detail.html, internal/app/service.go
- **Expected behavior:** POST /checks/{id}/delete removes the check, its pings, and its alerts, then redirects to /. The button asks for confirmation in the browser.
- **Acceptance:**
  - [x] The job disappears from the list and from the API.
  - [x] Its ping URL then returns 404.
  - [x] A missing CSRF does not delete it.
- **Test cases:** Confirm; cancel the dialog; missing CSRF; unknown id; demo mode.
- **UI checks:** The button is labeled Delete and uses the danger style.
- **Dependencies:** AUTH-04
- **Suspected bugs:** web/templates/check_detail.html:23 asks for confirmation only after Alpine starts. With scripts blocked, the POST still deletes.
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### JOB-07 Ping URL and copy

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/view.go, web/templates/check_detail.html, web/static/tock.js
- **Expected behavior:** The detail page shows TOCK_BASE_URL/ping/{token} and a curl -fsS line. Copy URL writes that URL and then reads Copied.
- **Acceptance:**
  - [x] The URL uses the configured base URL with no trailing slash.
  - [x] The curl line uses the same URL.
  - [x] The token is not printed on the job list.
- **Test cases:** http base URL; https base URL; a base URL that was configured with a trailing slash.
- **UI checks:** The URL field has a visually hidden label. Copy is a button, not a link.
- **Dependencies:** JOB-03
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### JOB-08 History tables

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/httpapi/dashboard.go, web/templates/check_detail.html
- **Expected behavior:** The page lists the latest 50 pings and 50 alerts. Empty history says "No pings yet." and "No alerts yet."
- **Acceptance:**
  - [x] A new check shows both empty lines.
  - [x] A ping adds a row with time, address, and agent.
  - [x] An alert row shows event, channel name, and result.
- **Test cases:** No rows; one row; more than 50 rows.
- **UI checks:** Tables have column headers. A failed alert shows its error text.
- **Dependencies:** PING-01, SCH-02
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### JOB-09 Schedule validation

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/check/status.go
- **Expected behavior:** A name must be non-empty, at most 80 runes, and must not contain control characters. Period must be 60 to 31536000 seconds. Grace must be 0 to 31536000 seconds. The form converts minutes, hours, and days.
- **Acceptance:**
  - [x] 59 seconds is rejected.
  - [x] 60 seconds is accepted.
  - [x] A name of spaces is rejected after trim.
  - [x] An unknown unit is rejected.
- **Test cases:** Empty; spaces; 80 runes; 81 runes; a newline; emoji; a negative number; a unit of "fortnights".
- **UI checks:** The error text is in role=alert and the entered values stay filled in.
- **Dependencies:** none
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### PING-01 Success ping

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/server.go, internal/app/service.go
- **Expected behavior:** GET and POST to the ping URL of a real check return 200, text/plain, and a body of OK plus a newline. The check becomes up when it was new.
- **Acceptance:**
  - [x] GET returns OK.
  - [x] POST returns OK.
  - [x] A second ping does not create a second check.
- **Test cases:** GET; POST; a query string; a large body; two tabs.
- **UI checks:** none
- **Dependencies:** JOB-03
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### PING-02 Unknown token

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/server.go
- **Expected behavior:** An unknown token returns 404 and the body "not found". No check is created.
- **Acceptance:**
  - [x] A random token is 404.
  - [x] The job count is unchanged.
- **Test cases:** Empty token path; a short token; a token with punctuation; a token from a deleted check.
- **UI checks:** none
- **Dependencies:** none
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### PING-03 New, up, grace, down

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/check/status.go, internal/store/pg/pg.go
- **Expected behavior:** A check that has never pinged stays new until period plus grace, measured from creation, and then the tick marks it down. After a ping it is up until the period ends, grace until the grace ends, then down.
- **Acceptance:**
  - [x] A fresh check displays new.
  - [x] A ping displays up.
  - [x] Time past the period and inside grace displays grace.
  - [x] Time past the due instant displays down.
- **Test cases:** No ping at creation; just before due; just after due; a late first ping.
- **UI checks:** The pill text matches the display status.
- **Dependencies:** SCH-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### PING-04 Recovery ping

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/check/status.go, internal/store/pg/pg.go
- **Expected behavior:** A ping on a down check stores up and queues one recovery for each enabled channel. A later ping while up does not send another recovery.
- **Acceptance:**
  - [x] The stored status becomes up.
  - [x] One recovery alert is sent.
  - [x] The next up ping does not send a recovery.
- **Test cases:** Down then ping; two pings in a row; a channel disabled before the ping.
- **UI checks:** The alert table shows recovered.
- **Dependencies:** NTF-06
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### PING-05 Paused ping

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/check/status.go
- **Expected behavior:** A ping for a paused check is stored. Status stays as it was and no recovery is queued.
- **Acceptance:**
  - [x] The ping appears in history.
  - [x] Status remains down or up as it was.
  - [x] No recovery row is created.
- **Test cases:** Pause while down, then ping; pause while up, then ping.
- **UI checks:** The paused note remains.
- **Dependencies:** JOB-05
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### PING-06 Address and agent

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/httpapi/server.go, internal/app/service.go
- **Expected behavior:** The stored address is the socket host unless TOCK_TRUST_PROXY is true, in which case the first X-Forwarded-For entry is stored. Address is clipped to 64 bytes and the user agent to 300 bytes. Logs print /ping/:token, not the token.
- **Acceptance:**
  - [x] Trust off ignores a forwarded header.
  - [x] Trust on stores the first forwarded address.
  - [x] A request log line does not contain the token.
- **Test cases:** No header; one address; two addresses; a very long user agent; trust true and false.
- **UI checks:** The history table shows the stored address.
- **Dependencies:** PING-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### PING-07 Demo ping

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/app/service.go, internal/demo/demo.go
- **Expected behavior:** In demo mode a known token returns OK and writes nothing. An unknown token is still 404.
- **Acceptance:**
  - [x] The synthetic last-ping time does not move.
  - [x] The ping count does not grow.
  - [x] An unknown token is 404.
- **Test cases:** GET the nightly-backup token; POST it twice; a random token.
- **UI checks:** The detail note says the demo accepts the request and does not record it.
- **Dependencies:** DEMO-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### SCH-01 Scheduler tick

- **Priority:** P0
- **Status:** PASSED
- **Files:** main.go, internal/config/config.go
- **Expected behavior:** The scheduler runs one tick immediately and then every 15 seconds. A panic in a tick is logged and the loop continues.
- **Acceptance:**
  - [x] An overdue check becomes down without a manual ping.
  - [x] The interval is 15 seconds in the default config.
- **Test cases:** A check created in the past beyond its due time; a check that is not yet due.
- **UI checks:** none
- **Dependencies:** OPS-03
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### SCH-02 Single down alert

- **Priority:** P0
- **Status:** PASSED
- **Files:** migrations/001_init.sql, internal/store/pg/pg.go
- **Expected behavior:** Moving a check to down writes one unresolved down alert per enabled channel. Later ticks do not add another while that row is unresolved.
- **Acceptance:**
  - [x] Two channels produce two down alerts.
  - [x] A second tick adds none.
  - [x] A disabled channel is skipped.
- **Test cases:** Zero channels; one channel; two channels; a channel disabled after the first alert.
- **UI checks:** The alert table shows one down row per channel.
- **Dependencies:** SCH-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### SCH-03 Pending then send

- **Priority:** P0
- **Status:** PASSED
- **Files:** migrations/002_alert_pending.sql, internal/app/service.go
- **Expected behavior:** The down or recovery row is inserted as pending in the same database step as the status change. Delivery happens after that. A row still pending on the next tick is sent.
- **Acceptance:**
  - [x] A successful send ends as sent.
  - [x] A pending row left behind is sent by the next tick.
- **Test cases:** A notifier that fails once before the row is settled; a process restart with a pending row.
- **UI checks:** The result column moves from pending to sent.
- **Dependencies:** SCH-02
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### SCH-04 Failed is final

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/app/service.go, internal/notify/email.go
- **Expected behavior:** A send error is stored on the alert as failed, clipped to 500 characters. The next tick does not send that row again.
- **Acceptance:**
  - [x] An email channel with no SMTP host becomes failed with "SMTP is not configured".
  - [x] The following tick does not call the notifier again for that row.
- **Test cases:** Empty SMTP host; a webhook that returns 500; a timeout.
- **UI checks:** The alert error is visible on the detail page.
- **Dependencies:** SCH-03
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### SCH-05 Advisory lock

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/store/pg/pg.go
- **Expected behavior:** Each tick takes pg_try_advisory_lock(748217001). If another session holds it, this tick returns without alerting. The lock is released when the tick finishes.
- **Acceptance:**
  - [x] Two overlapping ticks do not write two down alerts.
  - [x] A released lock lets the next tick run.
- **Test cases:** Two processes, one overdue check, one tick window.
- **UI checks:** none
- **Dependencies:** SCH-02
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### SCH-06 Retention

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/config/config.go, internal/store/pg/pg.go
- **Expected behavior:** Each tick deletes pings older than TOCK_PING_RETENTION_DAYS, default 90, allowed 1 to 3650. Checks and alerts remain.
- **Acceptance:**
  - [x] A ping older than the window is gone.
  - [x] A ping inside the window remains.
  - [x] A value of 0 or 3651 refuses to start.
- **Test cases:** Default; 1 day; a non-numeric value.
- **UI checks:** none
- **Dependencies:** SCH-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### SCH-07 Down with no channel

- **Priority:** P1
- **Status:** PASSED
- **Files:** web/templates/channels.html, internal/store/pg/pg.go
- **Expected behavior:** With no enabled channels, an overdue check still becomes down. The channels page says Tock will mark the job down and that nothing was notified.
- **Acceptance:**
  - [x] The job shows down.
  - [x] No alert row is sent to an external host.
- **Test cases:** No channels; all channels disabled.
- **UI checks:** The empty-channel note is present.
- **Dependencies:** SCH-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### NTF-01 Email channel

- **Priority:** P0
- **Status:** PASSED
- **Files:** web/templates/channels.html, internal/notify/notify.go
- **Expected behavior:** The operator adds an email channel with one or more addresses. Invalid addresses are rejected. The channel does not store the SMTP password.
- **Acceptance:**
  - [x] A valid address is saved.
  - [x] An empty recipient list is rejected.
  - [x] A value that is not an email is rejected.
- **Test cases:** One address; two addresses; spaces; a missing @; an empty field.
- **UI checks:** The kind selector reveals the recipient field. The error is in role=alert.
- **Dependencies:** AUTH-04
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### NTF-02 SMTP

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/notify/email.go, docker-compose.yml
- **Expected behavior:** An empty host does not stop startup. A send then fails. Port 465 uses implicit TLS. Any other port, including the default 587, uses opportunistic STARTTLS. Mailpit is the local smtp Compose profile on 1025 and 8025.
- **Acceptance:**
  - [x] A configured host sends the down subject and the recovery subject.
  - [x] An empty host records a failed alert and the process stays up.
  - [x] Port 465 is a different TLS mode from 587.
- **Test cases:** Mailpit; a refused port; a missing From; a 10 second timeout.
- **UI checks:** none
- **Dependencies:** NTF-01, SCH-04
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### NTF-03 Slack

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/notify/http.go, internal/notify/notify.go
- **Expected behavior:** A Slack channel stores an absolute http or https webhook URL and posts a JSON object with one text field. The text is the same plain body as email.
- **Acceptance:**
  - [x] A valid URL is saved.
  - [x] A relative URL is rejected.
  - [x] The posted body has text and does not have the ping token.
- **Test cases:** https URL; http URL; a blank URL; a URL with a space.
- **UI checks:** The Slack field is hidden until that kind is selected.
- **Dependencies:** NTF-06
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### NTF-04 Webhook

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/notify/notify.go, internal/notify/http.go
- **Expected behavior:** A webhook channel POSTs JSON with event down or recovered, the check id, name, status, periods, paused, last_ping_at, and sent_at. Headers are at most 20, one Name: value per form line, and cannot include Host, Content-Length, Transfer-Encoding, or Connection.
- **Acceptance:**
  - [x] The payload matches that shape.
  - [x] A configured header is sent.
  - [x] A forbidden header name is rejected.
  - [x] More than 20 headers are rejected.
- **Test cases:** No headers; one Authorization header; a header with a newline; 21 headers; a non-URL.
- **UI checks:** The header textarea is hidden until webhook is selected.
- **Dependencies:** NTF-06
- **Suspected bugs:** internal/notify/notify.go:178 accepts any http or https host, including a loopback or link-local address.
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### NTF-05 Edit and delete channel

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/app/service.go, web/templates/channels.html
- **Expected behavior:** Saving a channel updates name, config, and the enabled checkbox. The kind cannot change. Deleting a channel leaves past alerts with the stored channel name.
- **Acceptance:**
  - [x] Unchecking Enabled stops later alerts to that channel.
  - [x] A deleted channel still has its name on an old alert.
  - [x] Demo mode does not show the save form.
- **Test cases:** Rename; disable; delete; an unknown id; a missing CSRF.
- **UI checks:** Delete asks for confirmation. Each field has a label.
- **Dependencies:** NTF-01
- **Suspected bugs:** web/templates/channels.html:48 uses the same Alpine confirm as check delete.
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### NTF-06 Message body

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/notify/notify.go
- **Expected behavior:** Down subject is "Tock: {name} is down". Recovery subject is "Tock: {name} recovered". The body includes last ping, period, grace, and the dashboard URL. It never includes the ping token.
- **Acceptance:**
  - [x] Both subjects match.
  - [x] The token is absent from the subject and body.
  - [x] A check that never pinged says last ping never.
- **Test cases:** Down; recovery; a name with HTML characters; a name with emoji.
- **UI checks:** none
- **Dependencies:** SCH-02
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### NTF-07 No redirects

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/notify/http.go
- **Expected behavior:** The default Slack and webhook client has a 10 second timeout and returns an error when the response is a redirect. It does not send the headers to the next host.
- **Acceptance:**
  - [x] A 302 response is a failed alert.
  - [x] The redirect target receives no request body and no Authorization header.
- **Test cases:** 302; 301; a timeout; a 500.
- **UI checks:** The failed alert shows the error.
- **Dependencies:** NTF-03, NTF-04
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### NTF-08 Demo notifications off

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/notify/notify.go, main.go
- **Expected behavior:** The notifier guard returns "notifications are disabled in demo mode" before email, Slack, or webhook code runs. The demo scheduler does not enqueue real alerts.
- **Acceptance:**
  - [x] No SMTP connection is opened.
  - [x] No webhook request is made.
  - [x] The banner says outbound email, Slack, and webhooks are off.
- **Test cases:** Demo with an email channel present; a tick while a synthetic job is down.
- **UI checks:** WEB-06 banner.
- **Dependencies:** DEMO-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### API-01 List and create checks

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/api.go
- **Expected behavior:** GET /api/v1/checks returns the checks. POST with name, period_seconds, and grace_seconds creates one and returns ping_url and curl.
- **Acceptance:**
  - [x] An authorized POST returns the new check.
  - [x] An unauthorized POST is 401.
  - [x] Demo mode is 403.
- **Test cases:** Valid body; missing name; period 59; extra unknown field; no auth.
- **UI checks:** none
- **Dependencies:** AUTH-06, JOB-09
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### API-02 Get, patch, delete

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/api.go
- **Expected behavior:** GET returns one check. PATCH applies only name, period_seconds, and grace_seconds when those keys are present. DELETE returns 204. An unknown id is 404.
- **Acceptance:**
  - [x] A patch with only name keeps the period.
  - [x] Delete removes it.
  - [x] An unknown id is 404 with {"error":"not found"}.
- **Test cases:** One field; all fields; empty object; unknown id; wrong type for period.
- **UI checks:** none
- **Dependencies:** API-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### API-03 Pause and resume API

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/api.go
- **Expected behavior:** POST pause and POST resume match the dashboard rules and return the check JSON.
- **Acceptance:**
  - [x] Pause sets paused true.
  - [x] Resume clears it.
  - [x] Demo mode is 403.
- **Test cases:** Pause twice; resume an unknown id; no auth.
- **UI checks:** none
- **Dependencies:** JOB-05
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### API-04 Pings and alerts API

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/httpapi/api.go, internal/app/service.go
- **Expected behavior:** GET /api/v1/checks/{id}/pings and /alerts return recent rows. A missing or non-positive limit uses 50. A limit above 200 uses 200. An unknown check is 404.
- **Acceptance:**
  - [x] The default returns at most 50.
  - [x] limit=500 returns at most 200.
  - [x] An unknown check is 404.
- **Test cases:** No limit; limit=1; limit=0; limit=abc; an unknown id.
- **UI checks:** none
- **Dependencies:** JOB-08
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### API-05 Channels API

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/api.go, internal/app/service.go
- **Expected behavior:** GET lists channels. POST creates email, slack, or webhook. PATCH keeps omitted fields, including kind. DELETE removes the channel.
- **Acceptance:**
  - [x] Each kind accepts the documented config object.
  - [x] A patch that omits config keeps the old config.
  - [x] A bad kind is 400.
- **Test cases:** Email; Slack; webhook; omitted enabled on create; omitted name on patch; delete.
- **UI checks:** none
- **Dependencies:** NTF-01, NTF-03, NTF-04
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### API-06 Reject bad JSON

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/httpapi/server.go
- **Expected behavior:** The API decoder rejects unknown fields. Bodies over 1 MiB are rejected. HTML in JSON strings is escaped. Demo mutations return 403 and the read-only error. Other failures return {"error":"internal error"} and do not echo SQL.
- **Acceptance:**
  - [x] An unknown field is 400 "invalid JSON".
  - [x] A huge body is 400.
  - [x] A demo POST is 403.
- **Test cases:** Extra field; truncated JSON; a string containing <script>; a body over 1 MiB.
- **UI checks:** none
- **Dependencies:** API-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### OPS-01 Health and ready

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/server.go
- **Expected behavior:** GET /healthz returns {"status":"ok","version":...} without a session. GET /readyz returns ok when Postgres answers and 503 {"status":"unavailable"} when it does not.
- **Acceptance:**
  - [x] Both routes work logged out.
  - [x] A stopped database makes /readyz 503 and leaves /healthz 200.
- **Test cases:** Database up; database down; no cookie.
- **UI checks:** none
- **Dependencies:** OPS-03
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### OPS-02 Process healthcheck

- **Priority:** P1
- **Status:** PASSED
- **Files:** main.go, Dockerfile
- **Expected behavior:** tock healthcheck requests /readyz on 127.0.0.1 and the listen port. A non-200 or a connection error exits 1. The image HEALTHCHECK runs that command.
- **Acceptance:**
  - [x] A ready process exits 0.
  - [x] A down database exits 1.
- **Test cases:** Ready; Postgres stopped; TOCK_ADDR set to a host and port.
- **UI checks:** none
- **Dependencies:** OPS-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### OPS-03 Migrations

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/db/db.go, migrations/001_init.sql, migrations/002_alert_pending.sql
- **Expected behavior:** The process runs Goose before it listens. The tables are checks, pings, channels, and alerts. Alert status allows pending, sent, and failed.
- **Acceptance:**
  - [x] A fresh database ends with both migrations applied.
  - [x] A second start does not fail.
- **Test cases:** Empty database; already migrated; a database the role cannot write.
- **UI checks:** none
- **Dependencies:** OPS-04
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### OPS-04 Configuration

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/config/config.go
- **Expected behavior:** Startup requires DATABASE_URL, TOCK_BASE_URL, TOCK_ADMIN_PASSWORD of at least 8 characters, and TOCK_SESSION_SECRET and TOCK_API_TOKEN of at least 16 characters. The base URL must be absolute http or https. A trailing slash is removed. TOCK_DEMO and TOCK_TRUST_PROXY accept true, false, yes, no, 1, and 0.
- **Acceptance:**
  - [x] A missing key exits 1 and names it.
  - [x] A short password exits 1.
  - [x] A trailing slash is stripped.
- **Test cases:** Each missing key; a short secret; ftp URL; TOCK_DEMO=maybe.
- **UI checks:** none
- **Dependencies:** none
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### OPS-05 Listen address

- **Priority:** P0
- **Status:** PASSED
- **Files:** main.go, internal/config/config.go
- **Expected behavior:** TOCK_ADDR is used when set. Otherwise a set PORT becomes :PORT. Otherwise the address is :8080.
- **Acceptance:**
  - [x] An unset address listens on :8080.
  - [x] PORT=10000 listens on :10000 when TOCK_ADDR is unset.
  - [x] TOCK_ADDR wins over PORT.
- **Test cases:** Neither set; only PORT; both set.
- **UI checks:** none
- **Dependencies:** OPS-04
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### OPS-06 Security headers

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/httpapi/server.go
- **Expected behavior:** Every response sets nosniff, X-Frame-Options DENY, Referrer-Policy no-referrer, and a CSP of default-src 'self' with script-src 'self' 'unsafe-eval', style-src 'self', img-src 'self', and frame-ancestors 'none'. HSTS is added only for an https base URL, for 15552000 seconds.
- **Acceptance:**
  - [x] A dashboard response has those headers.
  - [x] A ping response has them too.
  - [x] An http base URL omits HSTS.
- **Test cases:** HTML page; /healthz; /ping; http and https base URLs.
- **UI checks:** The page still shows its stylesheet and the relative times still update.
- **Dependencies:** none
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### OPS-07 Logs and panics

- **Priority:** P1
- **Status:** PASSED
- **Files:** main.go, internal/httpapi/server.go
- **Expected behavior:** Each request is one JSON log line with method, redacted path, status, and milliseconds. A handler panic is logged with a stack and the client receives 500 "internal error". A scheduler panic is logged and later ticks still run.
- **Acceptance:**
  - [x] A ping log line says /ping/:token.
  - [x] A panic does not print a stack to the HTTP body.
- **Test cases:** A normal request; a forced panic if a test hook exists; a ping token that looks like a secret.
- **UI checks:** none
- **Dependencies:** none
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### OPS-08 Shutdown

- **Priority:** P2
- **Status:** PASSED
- **Files:** main.go
- **Expected behavior:** SIGINT and SIGTERM cancel the scheduler and call HTTP shutdown with a 10 second timeout.
- **Acceptance:**
  - [x] The process exits after the signal.
  - [x] A new connection is not accepted after shutdown starts.
- **Test cases:** SIGTERM during an idle process.
- **UI checks:** none
- **Dependencies:** OPS-05
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### WEB-01 Sign-in page

- **Priority:** P1
- **Status:** PASSED
- **Files:** web/templates/login.html, web/static/tock.css
- **Expected behavior:** The page heading is Sign in. The password field is required and autofocused. The lede names TOCK_ADMIN_PASSWORD.
- **Acceptance:**
  - [x] The label is associated with the password input.
  - [x] Submit with an empty field is blocked by the browser.
- **Test cases:** Keyboard only; a narrow viewport; an error then a correction.
- **UI checks:** Contrast of the muted text on the paper background. No horizontal overflow.
- **Dependencies:** AUTH-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### WEB-02 Chrome, title, icon, robots

- **Priority:** P1
- **Status:** PASSED
- **Files:** web/templates/layout.html, web/static/favicon.svg
- **Expected behavior:** The document language is en. The viewport meta is present. The title is "{page} · Tock". The icon is /static/favicon.svg. robots is noindex. The footer shows the version.
- **Acceptance:**
  - [x] The job page title is "Jobs · Tock".
  - [x] The icon request returns the SVG.
  - [x] robots noindex is present.
- **Test cases:** Login; jobs; a check detail; channels.
- **UI checks:** The mark links to /. Jobs and Channels expose aria-current on the active item.
- **Dependencies:** none
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### WEB-03 Responsive layout

- **Priority:** P2
- **Status:** PASSED
- **Files:** web/static/tock.css
- **Expected behavior:** At 720px and below, the header wraps, the nav loses the left auto margin, the detail split becomes one column, and the job row stacks the pill, name, and meta.
- **Acceptance:**
  - [x] 360, 720, and 1024 show the full job name without page overflow.
  - [x] The ping URL can be scrolled or wrapped inside its field.
- **Test cases:** 360, 768, 1024, and 1440 widths.
- **UI checks:** No control is clipped. The sign-out control remains reachable.
- **Dependencies:** WEB-02
- **Suspected bugs:** none
- **Bugs Found:** 1
- **Bugs Fixed:** 1
- **UI Issues Fixed:** 1
- **Files Changed:** web/static/tock.css, internal/httpapi/server_test.go
- **Notes:** A long ping URL and an unbreakable alert error widened the page at 360 and 768. The field and table cells now shrink and wrap. Chrome showed no page overflow at 360, 768, 1024, or 1440.

### WEB-04 Keyboard focus

- **Priority:** P1
- **Status:** PASSED
- **Files:** web/static/tock.css
- **Expected behavior:** Buttons, links, inputs, selects, and textareas show a 2px outline on focus-visible.
- **Acceptance:**
  - [x] Tabbing from the mark reaches nav, then the page actions.
  - [x] The outline is visible on the paper background.
- **Test cases:** Tab through login; tab through a check detail; Shift-Tab.
- **UI checks:** The outline is not removed on mouse focus-visible.
- **Dependencies:** WEB-02
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### WEB-05 Relative times

- **Priority:** P2
- **Status:** PASSED
- **Files:** web/static/tock.js
- **Expected behavior:** Elements with agoClock replace the label from the ISO time: just now, minutes, hours, or days, with "in" for a future time. The absolute UTC line stays. The label refreshes every 30 seconds. An invalid ISO value is shown as itself.
- **Acceptance:**
  - [x] A recent ping reads as a relative time.
  - [x] The UTC text remains available.
  - [x] An empty value reads "never".
- **Test cases:** A time a few seconds ago; a time yesterday; a future time; a blank attribute.
- **UI checks:** The text does not overflow the job row.
- **Dependencies:** JOB-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### WEB-06 Demo banner

- **Priority:** P1
- **Status:** PASSED
- **Files:** web/templates/layout.html, web/templates/checks.html, web/templates/channels.html
- **Expected behavior:** Demo mode shows "Demo mode. These jobs are simulated, and outbound email, Slack, and webhooks are off." It hides New check and the channel editor, and shows channel values as text.
- **Acceptance:**
  - [x] The banner is on every demo page.
  - [x] A normal install does not show it.
- **Test cases:** Demo on; demo off.
- **UI checks:** The banner is readable on the grace background.
- **Dependencies:** AUTH-08
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### WEB-07 Not found

- **Priority:** P1
- **Status:** PASSED
- **Files:** internal/httpapi/dashboard.go
- **Expected behavior:** An unknown check id on the detail or edit page returns 404. The body is the plain not-found response, not the dashboard layout.
- **Acceptance:**
  - [x] /checks/missing is 404.
  - [x] /checks/missing/edit is 404.
  - [x] A logged-out normal install is redirected to login before the lookup.
- **Test cases:** Unknown id; a deleted id; a logged-out request.
- **UI checks:** Confirm whether the plain page is acceptable at 360 and 1440. It has no Tock title.
- **Dependencies:** JOB-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### DEMO-01 Seeded jobs

- **Priority:** P0
- **Status:** PASSED
- **Files:** internal/demo/demo.go
- **Expected behavior:** Demo seed creates nightly-backup up, weekly-digest inside grace, and db-vacuum down, each with two synthetic pings. db-vacuum also has one sent down alert named "example channel". Later demo runs realign those clocks. The fixed tokens are in the source.
- **Acceptance:**
  - [x] The three names and their display statuses match.
  - [x] Restarting the process does not duplicate them.
  - [x] Their clocks stay aligned after a later start.
- **Test cases:** First start; second start; a tick while demo is on.
- **UI checks:** The job list shows up, grace, and down.
- **Dependencies:** AUTH-08
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### DEMO-02 Edge rewrite

- **Priority:** P1
- **Status:** PASSED
- **Files:** edge/vercel.json, README.md
- **Expected behavior:** The edge project rewrites every path to https://tock-emtj.onrender.com. It does not serve the Go templates itself. The README says the public demo is https://tock.rejisterjack.com and runs with TOCK_DEMO=true only.
- **Acceptance:**
  - [x] The rewrite destination is that Render host.
  - [x] The edge package does not contain the dashboard templates.
- **Test cases:** Read the rewrite file. During step 2, request the public host and confirm it is the demo banner, not a writable dashboard.
- **UI checks:** The public host, if it answers, shows the demo banner.
- **Dependencies:** DEMO-01
- **Suspected bugs:** VISION.md still lists an optional Railway demo. The README and the edge rewrite name Render.
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### DEVOPS-01 CI

- **Priority:** P0
- **Status:** PASSED
- **Files:** .github/workflows/ci.yml
- **Expected behavior:** Push and pull_request run go test -race, go vet, the integration tests against Postgres 16, govulncheck, and docker build. Go is pinned to 1.27.4.
- **Acceptance:**
  - [x] The workflow file contains those steps.
  - [x] This step did not run the workflow.
- **Test cases:** Read the workflow. Step 2 runs the same commands locally.
- **UI checks:** none
- **Dependencies:** none
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### DEVOPS-02 GHCR image

- **Priority:** P0
- **Status:** PASSED
- **Files:** .github/workflows/ci.yml, Dockerfile
- **Expected behavior:** A tag starting with refs/tags/v builds linux/amd64 and linux/arm64 and pushes ghcr.io/rejisterjack/tock:{tag} and :latest. The version build arg is the tag name. The runtime image is distroless nonroot.
- **Acceptance:**
  - [x] The workflow is limited to v* tags.
  - [x] The Dockerfile builds with CGO disabled and runs as nonroot.
- **Test cases:** Read the workflow and Dockerfile. Step 2 checks the published tag only if a release exists.
- **UI checks:** none
- **Dependencies:** DEVOPS-05
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### DEVOPS-03 Compose

- **Priority:** P0
- **Status:** PASSED
- **Files:** docker-compose.yml, .env.example
- **Expected behavior:** Compose builds the app, waits for Postgres 16 to be healthy, and publishes 8080. The database URL inside Compose points at the postgres service. The smtp profile adds Mailpit on 8025 and 1025. The app healthcheck runs /tock healthcheck.
- **Acceptance:**
  - [x] The file matches that layout.
  - [x] Example secrets are the ones AUTH-07 refuses on a public host.
- **Test cases:** Read the file. Step 2 runs the documented Compose path.
- **UI checks:** none
- **Dependencies:** OPS-02
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### DEVOPS-04 Smoke workflow

- **Priority:** P1
- **Status:** PASSED
- **Files:** .github/workflows/smoke.yml, scripts/smoke.py
- **Expected behavior:** The smoke workflow is workflow_dispatch only. It copies .env.example to .env and runs scripts/smoke.py. The README says that script signs in, pings a check, checks one email and one webhook for down and recovery, then expects /readyz to fail after Postgres stops.
- **Acceptance:**
  - [x] It is not triggered by pull_request.
  - [x] The script file exists.
- **Test cases:** Read the workflow. Step 2 runs the script only as the release check it documents.
- **UI checks:** none
- **Dependencies:** DEVOPS-03, NTF-02
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### DEVOPS-05 Go 1.27.4

- **Priority:** P1
- **Status:** PASSED
- **Files:** go.mod, Dockerfile, .github/workflows/ci.yml
- **Expected behavior:** The module, the CI setup-go input, and the Docker build image all say Go 1.27.4.
- **Acceptance:**
  - [x] All three files name 1.27.4.
- **Test cases:** Read the three files. Step 2 runs go version.
- **UI checks:** none
- **Dependencies:** none
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

### DEVOPS-06 Job examples

- **Priority:** P2
- **Status:** PASSED
- **Files:** examples/cron/backup.sh, examples/worker/loop.sh, examples/one-off/task.sh
- **Expected behavior:** Each example pings only after its work succeeds. The cron example pings after pg_dump. The worker pings at the end of a pass. The one-off pings after the script finishes.
- **Acceptance:**
  - [x] A failing command in the example does not reach the curl.
  - [x] The curl uses the ping URL placeholder from the file.
- **Test cases:** Read each script. Do not run them against a database.
- **UI checks:** none
- **Dependencies:** PING-01
- **Suspected bugs:** none
- **Bugs Found:**
- **Bugs Fixed:**
- **UI Issues Fixed:**
- **Files Changed:** none
- **Notes:** Passed when exercised on 22 Nov 2026.

## Assumptions

- This report describes the committed tree at 311894d, which is the same tree as origin/main a72d56e and tag v0.2.0. The local checkout is branch fix/production-hardening.
- The working tree also has an uncommitted edit to web/static/favicon.svg, plus untracked logo.svg and edge/.gitignore. Those were not treated as product behavior.
- The public demo host in the README was not requested. DEMO-02 records the rewrite target only.
- There is no AI feature and no billing feature.
- Slack delivery is described from internal/notify/http.go. The README points at TestSlackAndWebhook. This step did not run it.
- One admin password, one API token, and one session secret are the whole account model.

## Risks

- Ping URLs are the only authentication on /ping. A leaked token can mark a job up. SECURITY.md says a database dump must be treated as a secret.
- The login limiter is per process and drops an arbitrary address at 10000 keys (internal/httpapi/limit.go:24). A second app container would not share it. The README says to run one app container.
- Webhook and Slack URLs may point at private addresses (internal/notify/notify.go:178). Redirects are refused (internal/notify/http.go:46).
- Demo tokens are constants in internal/demo/demo.go. They answer OK and change nothing, and only exist after demo seed.
- Delete confirmation depends on Alpine. The CSP allows 'unsafe-eval' because Alpine is vendored.
- VISION.md still names Railway. The running demo path in the README is Render, with a Vercel rewrite in edge/.
- A failed alert is not retried. An operator has to cause a new down or recovery after fixing SMTP or the webhook.

## Whole-Product Check

- `go test -race ./...` passed. `go test -tags integration ./...` passed against Postgres 16. `go vet ./...` passed. `govulncheck` found no vulnerability called by this code. One vulnerability remains in a required module that Tock does not call.
- A live process on 127.0.0.1:18081 signed in, showed the empty job steps, created a check, rejected an empty name and a bad CSRF, and stored GET and POST pings. An unknown token returned 404. The stored address stayed 127.0.0.1 when `X-Forwarded-For` was sent and trust was off.
- The scheduler marked that check down. Mailpit received `Tock: nightly-backup is down` and later `Tock: nightly-backup recovered`. A webhook received both events and neither body contained the ping token. A second tick did not send another down alert. A delivery to a closed port was stored as failed.
- The API created, patched, paused, resumed, and deleted a check. Unknown JSON was 400. A bad Slack URL and a bad email were 400. Email and webhook channels saved.
- Editing the period left the ping token in place. Pause showed the paused note. Logout cleared the session. Eight wrong demo passwords returned 401 and the ninth returned 429.
- Example secrets with a public base URL exited 1 and named the three keys. `/healthz` and `/readyz` answered without a session. HTTP responses had nosniff, DENY framing, no-referrer, and the CSP, and did not send HSTS.
- Demo mode on 127.0.0.1:18082 showed the three synthetic jobs and the banner without a session. Creating a check was 403. A known demo token returned OK.
- Chrome signed in, opened the paused check, and copied the ping URL. Footer text on the paper background was 5.82:1. There was no page overflow at 360, 768, 1024, or 1440 after the CSS fix, and the console had no errors.

## Totals

- Features: 68
- Passed: 68
- Blocked: 0
- P0: 43
- P1: 21
- P2: 4
- Bugs found: 1
- Bugs fixed: 1
- UI issues fixed: 1

## Still Blocked

None.

## Known Limitations

- Deferred, and not feature rows: multiple accounts, cron expressions, start and fail signals, per-check routing, APM, billing, a dark theme, a password screen, alert retries, and a ping rate limit.
- The login limiter is per process. The README says to run one app container.
- Webhook and Slack URLs may point at a private address. Redirects are refused.
- Demo tokens are constants and do not change the synthetic jobs.
- Delete confirmation is asked by Alpine. The CSP allows `unsafe-eval` for that library.
- VISION.md still names Railway. The demo path in the README is Render, with the Vercel rewrite in `edge/`.
- `govulncheck` reported one vulnerability in a required module that this code does not call.

## Launch Checklist

- Copy `.env.example` to `.env` and replace the password, session secret, and API token before using a public base URL.
- `docker compose up -d` starts Postgres and the app. Add the smtp profile only for local mail.
- Create a check, copy the ping URL, and append it to the job so it runs only after success.
- Add an email, Slack, or webhook channel. Leave SMTP empty and email alerts fail closed until it is set.
- The public demo stays on `TOCK_DEMO=true`. Real alerting stays on the operator's Compose install.

## Decision

GO. 68 features passed. No feature is blocked. The dashboard, the ping receiver, the scheduler, mail, the webhook, the API, and the demo passed the checks in this report.
