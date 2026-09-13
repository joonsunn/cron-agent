# cron-agent

Recurring agent jobs on cron schedules, executed headlessly through `opencode`, with a small dashboard for health and history.

One Go binary owns the scheduling loop and the HTTP API. No system daemons. All mutable state lives in `./data`, outside server code.

## Prerequisites

- `opencode` on `PATH`, checked at boot
- Go 1.23+, Node 20+, pnpm 9+

## Quickstart

```sh
make setup
make dev
```

Open the dashboard at `http://localhost:5173`. The API runs on port 8080.

The example job ships disabled, so setup spends no model calls. To schedule real work, copy `data/jobs/example.yaml` to a new file, point it at a prompt in `data/prompts`, and set `enabled: true`. Job files apply on the next 10s tick, no restart needed.

```sh
make build   # web dashboard plus server binary at bin/cron-agent
make run     # serve the production build against ./data
make test    # Go tests plus web typecheck
```

## How it works

A ticker wakes every 10 seconds, reloads job files, and fires due jobs with a single-writer lease, so overlaps skip instead of stacking. The runner executes `opencode run` with the job prompt, streams output to a per-run log, and refreshes the job STATUS.md on completion.

Persistence is split by purpose. Markdown under `data/memory` holds live state the next run reads. SQLite holds append-only run history the dashboard queries. Job definitions and prompts are plain files, safe to version.

Headless runs cannot approve anything, and an `ask` permission hangs forever waiting for input. `data/opencode.json` pins permissions for runs: tools inside the data dir are allowed, while `external_directory`, `doom_loop`, and `question` are denied so approval needs fail fast. Job load warns when a prompt references absolute paths or parent traversal.

## Configuration

Copy `.env.example` to `.env` at the repo root for local dev. Only `DATA_DIR`, `PORT`, and `API_TOKEN` load. Precedence is make flags, then `.env`, then defaults. Never commit `.env`.

For a shipped binary, place a `.env` next to the binary, or pass `--env <path>`. The binary also reads `./.env` from its working dir. Exported env vars always win.

## Layout

- `apps/server` scheduler loop, opencode runner, SQLite store, HTTP API
- `apps/web` React dashboard, served by Go in prod and Vite in dev
- `packages/shared` API types shared by server JSON and web client
- `data` jobs, prompts, run law, permission pins, memory, db, logs
- `SPEC.md` normative behavior, `PLAN.md` build order, `docs/CONTEXT.md` decisions
