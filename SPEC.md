# cron-agent spec

## Goal

Run recurring agent jobs from one self-contained service. Each job runs on a cron schedule through the `opencode` harness. A small dashboard shows job health and history.

## Non-goals

No OS daemons, no managed queue, no multi-tenant auth, no live agent chat in v1.

## Prerequisites

`opencode` is installed and on `PATH`. The server refuses to start without it.

Go 1.23+, Node 20+, pnpm 9+.

## Architecture

One Go binary owns the scheduling loop and the HTTP API. The process starts both together and stops both together. No launchd, systemd, or cron entries are required.

A React dashboard is embedded in the Go binary in prod and served by Vite dev server in local dev.

All mutable state lives in one data dir outside server code. Default `./data`, resolved relative to the directory the binary runs from, overridable by `--data` flag or `DATA_DIR` env. Server code stays read only.

## Job registration

A job is one YAML file under `<data>/jobs/*.yaml`. Adding a file registers a schedule. Deleting it unregisters. Editing it reschedules on next tick without restart.

Fields:

```yaml
name: daily-news
schedule: "0 9 * * *"
promptFile: prompts/daily-news.md
timezone: UTC
timeoutMinutes: 10
enabled: true
notifyOnFailure: owner@example.com
```

`schedule` is a standard 5-field cron string. All schedules evaluate in the job `timezone`, default UTC. `promptFile` resolves relative to the data dir.

## Persistence

Markdown holds live state the agent reads next. Location `<data>/memory/<job>/STATUS.md` plus rolling summaries. Keep STATUS under 40 lines so prompts stay small.

SQLite holds append-only history. Tables `jobs`, `runs`. A run records scheduled time, start, finish, exit code, output path, trigger source. History uses insert only, never update in place.

`<data>/jobs` holds definitions. `<data>/memory` holds markdown. `<data>/cronagent.db` holds SQLite. `<data>/logs/<job>/<runID>.log` holds raw output. DB and logs are gitignored. Definitions and prompts can be versioned.

A fresh data dir is seeded on boot from templates embedded in the binary: example job and prompt, permission pins, run law. Seeding never overwrites existing files, so a copied binary self-provisions a working setup anywhere.

## Scheduler semantics

Ticker wakes every 10 seconds. It reloads job files, computes next run with a cron parser, and compares against SQLite lease state.

A job fires only when due, enabled, and unlocked. The runner claims a lease before exec so two ticks never double run. Overlapping runs of the same job are skipped and logged as skipped.

At most 3 runs execute at once by default, overridable by `--concurrent` flag or `MAX_CONCURRENT` env. Excess firings wait in an in-memory FIFO queue instead of starting new processes. A job that fires again while already queued or running reports busy instead of stacking a duplicate.

Timeout kills the `opencode` child and marks the run timed out. Failures record exit code and last 100 lines for the dashboard. Three consecutive failures surface a badge on the dashboard.

Manual trigger and pause are API driven and also respect the single-writer lease.

## Runner semantics

Runner executes `opencode run <prompt> --working-dir <data>` with the job timeout as context deadline. Stdout and stderr stream to the run log file and back to the markdown STATUS on completion.

Runner never edits server code. It may write only inside the data dir. Summarizer jobs get read-only intent prompts. Actor jobs must be explicitly marked and keep an approval step outside v1.

Project config lives at `<data>/opencode.json` and pins opencode permissions. Tools inside the data dir are allowed, while `external_directory`, `doom_loop`, and `question` are denied so approval needs fail fast instead of hanging a headless run. Job load warns when a prompt references absolute paths or parent traversal.

## HTTP API

- `GET /api/health` returns ok plus opencode presence.
- `GET /api/jobs` returns definitions plus next run, last status, consecutive failures.
- `GET /api/runs?job=<name>&limit=50` returns recent runs newest first.
- `GET /api/runs/<id>/log` returns the run log.
- `POST /api/jobs/<name>/trigger` queues an immediate run.
- `POST /api/jobs/<name>/pause` and `/resume` toggle enabled.

Dashboard v1 is read only except trigger, pause, resume.

## Dashboard

Shows job list with next run and last status. Shows run history with exit codes and durations. Shows log view per run. Polls every 10 seconds. No streaming in v1.

## Failure handling

Lease refreshes every 30 seconds during a run. A dead host leaves an expired lease so the next tick can reclaim. Missed runs log a warning, they never backfill silently.

## Security

Local dev runs without auth. Any exposed deployment sits behind token auth (`API_TOKEN` bearer). Prompts are treated as untrusted input for rendering, escaped in the dashboard.

## Verification

`make build` compiles server and web. `make test` runs Go tests and web typecheck. `make run` boots the server against `./data` with the example job disabled by default so nothing calls the model until the user enables it.
