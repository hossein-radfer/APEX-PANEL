# Running ApexPanel as a systemd service (Linux)

## Why this is necessary

Running the panel with `nohup go run ./api/cmd/main.go &` from an SSH
session is not safe for production, even with `nohup`. `nohup` only blocks
`SIGHUP` -- it does **not** protect a process from `systemd-logind`'s
session cleanup. On most modern Linux distros, `logind`'s
`KillUserProcesses=yes` (the default) kills every process still owned by a
login session's cgroup the moment that session ends (SSH disconnects,
terminal closes, network blip, etc.), regardless of `nohup` or `&`. That
process receives a clean `SIGTERM`, shuts down gracefully (which is why
there's no crash/OOM evidence in the logs -- it isn't crashing, it's being
asked to stop), and nothing brings it back up.

Running as a systemd service fixes this at the root: the process belongs to
systemd's own scope, completely independent of any login session, so an SSH
disconnect can never reach it. `Restart=always` also means any other kind
of unexpected exit self-heals within seconds.

This has been verified in code: there is no timer, watchdog, or shutdown
logic anywhere in this codebase that calls `os.Exit` or sends the process a
signal -- the only place `os.Exit`-adjacent behavior exists is Echo's own
internal `Fatal` path if the HTTP listener itself fails, which is logged
through the persistent log file (see below) and is a distinct, clearly
different failure mode from a clean `SIGTERM`.

## 1. Build a static binary

From the repository root:

```bash
git clone https://github.com/hossein-radfer/APEX-PANEL.git
cd mwp

# Build the frontend first -- the Go binary embeds ui/dist directly.
cd ui
npm install
npm run build
cd ..

# Build a single, statically-linked binary. CGO_ENABLED=0 avoids a runtime
# dependency on libc, so the binary runs on any Linux host without needing
# matching system libraries installed.
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mwp -ldflags="-s -w" ./api/cmd

# Install it system-wide.
sudo install -v -o root -g root -m 755 mwp /usr/local/bin/mwp
```

`-ldflags="-s -w"` strips debug symbols to reduce binary size; omit it if
you ever need to attach a debugger to a production binary.

If you're building on a different architecture than the deploy target
(e.g. building on an ARM Mac for an x86_64 VPS, or cross-compiling from
Windows as in this session), adjust `GOARCH` accordingly (`amd64`, `arm64`,
etc.) -- `GOOS=linux` and `CGO_ENABLED=0` cross-compile cleanly from any
host with no extra toolchain needed.

## 2. Install as a systemd service

### Option A -- automated (recommended)

`deploy/install.sh` does everything below (directories, `.env`, systemd
unit, start) in one step:

```bash
sudo ./deploy/install.sh --binary ./mwp \
  --license-key MWP-XXXX-XXXX-XXXX-XXXX \
  --license-public-key <public key printed by license-panel's install.sh>
```

If you don't have a license key yet, omit both `--license-key` and
`--license-public-key` -- the panel starts locked to its activation
screen, where you can activate it from the browser instead. Run
`./deploy/install.sh --help` for every option (custom admin
credentials, `--db-dialect postgres`, and the dev-only
`--license-server-url`/`--dev-mode` pair described below).

### Option B -- manual

```bash
sudo cp deploy/mwp.service /etc/systemd/system/mwp.service
sudo systemctl daemon-reload
sudo systemctl enable --now mwp
```

`enable` makes it start automatically on boot; `--now` also starts it
immediately. From this point on, **never start the panel with `nohup`/`&`
again** -- always use `systemctl start|stop|restart mwp`.

If you were previously running it via `nohup`, kill that process first so
there isn't a second instance fighting over the same port/database:

```bash
pkill -f 'mwp/cmd/main.go'   # or: pkill -f '/usr/local/bin/mwp', whichever was running
sudo systemctl restart mwp
```

## Check status / logs

```bash
sudo systemctl status mwp
sudo journalctl -u mwp -f
```

## Interactive management menu

Running the same binary with a `menu` argument opens a text-based
management menu, instead of starting the HTTP server:

```bash
mwp menu
```

From there you can check/start/stop/restart the systemd service, reset an
admin account's password (no old password required -- this is the
recovery path for a locked-out admin), change the panel's listen port
(writes the same `SystemConfig` override the web UI's settings page
writes to, so a restart is still required to apply it), view recent log
entries without leaving the terminal, and create or restore a database
backup. This never conflicts with the running service -- it opens its own
short-lived database connection per action and exits back to the shell
when you're done.

## Remote support access (no SSH required)

Under Settings &rarr; System &rarr; **Support Access** in the web panel, you
can generate a time-limited (24-hour) support token and share it with our
support team. This grants a read-only view of this panel's own log
entries through a dedicated endpoint -- nothing more. There is no SSH
access, no server credential, and no way to reach a shell or any other
panel data through this token. It auto-expires after 24 hours, and you can
revoke it immediately at any time from the same settings card.

## If the panel goes down and won't come back up

Run this block and send the full output -- it collects everything needed to
diagnose the cause in one place:

```bash
echo "=== systemctl status ===" 
sudo systemctl status mwp --no-pager -l

echo "=== last 300 journald lines ===" 
sudo journalctl -u mwp -n 300 --no-pager

echo "=== last 300 lines of the panel's own log file ===" 
tail -n 300 ~/.config/mwp/logs/mwp.log

echo "=== OOM kill check (kernel log) ===" 
sudo dmesg -T | grep -i -E 'oom|killed process' | tail -n 50

echo "=== current memory / disk state ===" 
free -h
df -h
```

If you're not running under systemd yet (see Install above), skip the first
two commands -- they'll just show "unit not found", which is fine.

### How to read it (for reference, you don't need to interpret this yourself --
just send the output)

The panel's own log file (`~/.config/mwp/logs/mwp.log`, default data dir --
use `$DATA_DIR` instead if that's set) now includes, on top of errors:

- A `resource heartbeat` line every 5 minutes with memory/goroutine counts.
  If these climb steadily right up until the log stops, that points at a
  memory leak / OOM kill.
- A `request` line for every HTTP request (method, URI, status, remote IP,
  latency). A flood of requests from unfamiliar IPs right before the log
  stops points at scanner/bot load overwhelming the process.
- A `received termination signal` line if something asked the process to
  stop cleanly (SIGTERM/SIGINT). **Once running under systemd**, this line
  appearing only means an intentional `systemctl stop`/`restart` happened --
  if you see it and didn't run either of those yourself, something else on
  the host (another script, a cron job, another admin) is stopping it, and
  `journalctl -u mwp` around that timestamp will show whether systemd itself
  requested the stop or something sent the signal directly.
- A `recovered from panic` line if something inside the panel itself
  panicked (a real code bug) -- this should now be rare, since panics in
  every background job/goroutine are caught and logged instead of crashing
  the process, but if you see one, that pinpoints the bug exactly.

## License binding and updates

`$DATA_DIR` (default `~/.config/mwp`) also stores `install-id` -- a
randomly generated identifier created on first run, used together with
this server's primary network interface to build the fingerprint this
install's license is bound to (see `api/license/fingerprint.go`). This
file is never touched by rebuilding or replacing the `mwp` binary, so a
normal update (steps 1-2 above) never affects licensing.

**Never delete, move, or copy `$DATA_DIR/install-id` to another server.**
Deleting it makes this install's license binding regenerate as if it were
a brand-new server, which the central license server (license-panel) will
treat as an unrecognized machine -- the panel will lock itself
immediately (by design, with no grace period; see the doc comment on
`LicenseService`) until support re-activates it. If you are migrating to
genuinely new hardware, contact support to have the old fingerprint
deactivated first -- do not try to carry `install-id` over manually.

## Testing against a license-panel without a TLS certificate yet

MWPanel talks to its license server over hardcoded HTTPS in production
(`api/config/config.go`'s `licenseServerBaseURL`) -- this is intentional
and cannot be overridden by a customer's own `.env`. For local/test
setups where your license-panel instance doesn't have a real certificate
yet, set BOTH of these in MWPanel's `.env` (or via `install.sh
--license-server-url ... --dev-mode`):

```
MODE=development
LICENSE_SERVER_URL=http://<test-server-ip-or-domain>:3131
```

`LICENSE_SERVER_URL` is silently ignored unless `MODE=development` is
also set -- a production install can never be redirected to a different
license server through this. Once your license-panel has a real
certificate (see its own `deploy/README.md` for the nginx+certbot flow),
remove both of these and MWPanel goes back to the hardcoded HTTPS
address.

## Restart after changing configuration (e.g. SERVER_PORT)

```bash
sudo systemctl restart mwp
```

If `SERVER_PORT` (or any other env var) is set via a `.env` file, place it
next to the binary or set `EnvironmentFile=/path/to/.env` in
`mwp.service` before `ExecStart`.
