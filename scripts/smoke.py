#!/usr/bin/env python3
"""Compose smoke test for a Tock release.

Signs in, pings a check, waits for one down alert and one recovery over email
and a webhook, then checks pause and /readyz. Run from anywhere; the script
finds the repository root.
"""

import json
import os
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

ROOT = Path(__file__).resolve().parents[1]
PROJECT = "tock-smoke"
BASE = "http://127.0.0.1:18080"
MAILPIT = "http://127.0.0.1:18025"
COMPOSE = [
    "docker", "compose", "-p", PROJECT,
    "-f", str(ROOT / "docker-compose.yml"),
    "-f", str(ROOT / "scripts" / "smoke.override.yml"),
    "--profile", "smtp",
]


def fail(msg):
    print("FAIL", msg, file=sys.stderr)
    sys.exit(1)


def compose(*args, check=True):
    result = subprocess.run([*COMPOSE, *args], cwd=ROOT, text=True, capture_output=True)
    if check and result.returncode != 0:
        fail(result.stderr.strip() or result.stdout.strip() or "compose failed")
    return result


def env_value(key):
    env = ROOT / ".env"
    if not env.exists():
        fail("copy .env.example to .env before running the smoke test")
    for line in env.read_text().splitlines():
        if not line or line.startswith("#") or "=" not in line:
            continue
        name, value = line.split("=", 1)
        if name == key:
            return value
    fail(f"{key} is missing from .env")


def http(method, url, data=None, headers=None, timeout=10):
    req = urllib.request.Request(url, data=data, headers=headers or {}, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as res:
            return res.status, dict(res.headers), res.read()
    except urllib.error.HTTPError as err:
        return err.code, dict(err.headers), err.read()


class Catcher(BaseHTTPRequestHandler):
    events = []

    def do_POST(self):
        n = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(n)
        Catcher.events.append({"path": self.path, "auth": self.headers.get("Authorization", ""), "body": body.decode()})
        self.send_response(204)
        self.end_headers()

    def log_message(self, fmt, *args):
        return


def wait_ready():
    for _ in range(30):
        try:
            code, _, _ = http("GET", BASE + "/readyz")
        except (urllib.error.URLError, TimeoutError, ConnectionError, OSError):
            time.sleep(1)
            continue
        if code == 200:
            return
        time.sleep(1)
    fail("Tock did not become ready")


def wait_status(check_id, token, want):
    deadline = time.time() + 25
    while time.time() < deadline:
        code, _, body = http("GET", f"{BASE}/api/v1/checks/{check_id}", headers={"Authorization": "Bearer " + token})
        if code == 200 and json.loads(body)["status"] == want:
            return
        time.sleep(1)
    fail(f"check did not become {want}")


def subjects(name):
    mail = json.load(urllib.request.urlopen(MAILPIT + "/api/v1/messages"))
    return [m["Subject"] for m in mail.get("messages", []) if name in m["Subject"]]


def main():
    password = env_value("TOCK_ADMIN_PASSWORD")
    token = env_value("TOCK_API_TOKEN")
    Catcher.events = []
    server = ThreadingHTTPServer(("0.0.0.0", 0), Catcher)
    port = server.server_address[1]
    Thread(target=server.serve_forever, daemon=True).start()

    def cleanup(*_):
        server.shutdown()
        compose("down", "-v", "--remove-orphans", check=False)
    signal.signal(signal.SIGINT, cleanup)
    signal.signal(signal.SIGTERM, cleanup)

    print("starting compose")
    compose("up", "-d", "--build")
    try:
        wait_ready()
        code, _, _ = http("GET", BASE + "/healthz")
        if code != 200:
            fail(f"healthz {code}")
        health = compose("exec", "-T", "app", "/tock", "healthcheck")
        if health.returncode != 0:
            fail("healthcheck failed while ready")

        created_body = json.dumps({"name": "smoke-backup", "period_seconds": 60, "grace_seconds": 0}).encode()
        code, _, body = http("POST", BASE + "/api/v1/checks", created_body, {
            "Authorization": "Bearer " + token, "Content-Type": "application/json",
        })
        if code != 201:
            fail(f"create check {code} {body[:200]!r}")
        created = json.loads(body)
        check_id = created["id"]
        ping = created["ping_url"].replace("http://127.0.0.1:18080", BASE)
        secret = ping.rsplit("/", 1)[-1]

        for kind, config in (
            ("email", {"to": ["ops@example.com"]}),
            ("webhook", {"url": f"http://host.docker.internal:{port}/hook", "headers": {"Authorization": "Bearer smoke"}}),
        ):
            payload = json.dumps({"kind": kind, "name": kind, "enabled": True, "config": config}).encode()
            code, _, body = http("POST", BASE + "/api/v1/channels", payload, {
                "Authorization": "Bearer " + token, "Content-Type": "application/json",
            })
            if code != 201:
                fail(f"create {kind} {code} {body[:200]!r}")

        code, _, text = http("GET", ping)
        if code != 200 or text != b"OK\n":
            fail(f"ping {code} {text!r}")
        code, _, body = http("GET", f"{BASE}/api/v1/checks/{check_id}", headers={"Authorization": "Bearer " + token})
        if json.loads(body)["status"] != "up":
            fail("ping did not mark the check up")

        # Sign in and confirm the dashboard shows the job.
        opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor())
        page = opener.open(BASE + "/login").read().decode()
        csrf = page.split('name="csrf" value="', 1)[1].split('"', 1)[0]
        data = urllib.parse.urlencode({"csrf": csrf, "password": password}).encode()
        res = opener.open(urllib.request.Request(BASE + "/login", data=data))
        home = res.read().decode()
        if "smoke-backup" not in home:
            fail("dashboard did not list the check after sign-in")

        compose("exec", "-T", "postgres", "psql", "-U", "tock", "-d", "tock", "-c",
                f"UPDATE checks SET last_ping_at = now() - interval '5 minutes' WHERE id = '{check_id}';")
        wait_status(check_id, token, "down")
        time.sleep(1)
        hooks = [json.loads(e["body"]) for e in Catcher.events if e["path"] == "/hook"]
        if [e["event"] for e in hooks] != ["down"]:
            fail(f"webhook events {[e['event'] for e in hooks]}")
        if Catcher.events[-1]["auth"] != "Bearer smoke" or secret in Catcher.events[-1]["body"]:
            fail("webhook auth header missing or ping token leaked")
        if subjects("smoke-backup").count("Tock: smoke-backup is down") != 1:
            fail(f"mailpit subjects {subjects('smoke-backup')}")

        code, _, _ = http("POST", ping)
        if code != 200:
            fail(f"recovery ping {code}")
        time.sleep(1)
        code, _, _ = http("POST", ping)
        time.sleep(1)
        hooks = [json.loads(e["body"])["event"] for e in Catcher.events if e["path"] == "/hook"]
        if hooks.count("down") != 1 or hooks.count("recovered") != 1:
            fail(f"webhook events {hooks}")
        mailed = subjects("smoke-backup")
        if mailed.count("Tock: smoke-backup is down") != 1 or mailed.count("Tock: smoke-backup recovered") != 1:
            fail(f"mailpit subjects {mailed}")

        code, _, body = http("POST", f"{BASE}/api/v1/checks/{check_id}/pause", b"{}", {
            "Authorization": "Bearer " + token, "Content-Type": "application/json",
        })
        if code != 200 or not json.loads(body)["paused"]:
            fail(f"pause {code} {body[:120]!r}")
        http("POST", ping)
        code, _, body = http("GET", f"{BASE}/api/v1/checks/{check_id}", headers={"Authorization": "Bearer " + token})
        got = json.loads(body)
        if not got["paused"] or got["status"] != "up":
            fail(f"paused ping changed the check: {got['status']} paused={got['paused']}")
        code, _, _ = http("POST", f"{BASE}/api/v1/checks/{check_id}/resume", b"{}", {
            "Authorization": "Bearer " + token, "Content-Type": "application/json",
        })
        if code != 200:
            fail(f"resume {code}")
        code, _, _ = http("DELETE", f"{BASE}/api/v1/checks/{check_id}", headers={"Authorization": "Bearer " + token})
        if code != 204:
            fail(f"delete {code}")

        print("stopping postgres")
        compose("stop", "postgres")
        deadline = time.time() + 20
        saw_down = False
        while time.time() < deadline:
            try:
                code, _, _ = http("GET", BASE + "/readyz", timeout=3)
            except (urllib.error.URLError, TimeoutError):
                time.sleep(1)
                continue
            if code == 503:
                saw_down = True
                break
            time.sleep(1)
        if not saw_down:
            fail("readyz stayed ready while postgres was stopped")
        down = compose("exec", "-T", "app", "/tock", "healthcheck", check=False)
        if down.returncode == 0:
            fail("healthcheck succeeded while postgres was stopped")
        compose("start", "postgres")
        wait_ready()
        up = compose("exec", "-T", "app", "/tock", "healthcheck")
        if up.returncode != 0:
            fail("healthcheck failed after postgres returned")
        print("smoke passed")
    finally:
        cleanup()


if __name__ == "__main__":
    os.chdir(ROOT)
    main()
