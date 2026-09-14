# cron-agent

Recurring agent jobs on cron schedules, executed headlessly through `opencode`, with a small dashboard for health and history.

One Go binary owns the scheduling loop and the HTTP API. No system daemons. All mutable state lives in `./data`, outside server code. Relative paths resolve from the directory you run the binary in, so start it from the directory that should hold state.

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
make build   # dashboard embedded plus server binary at bin/cron-agent
make run     # serve the production build against ./data
make test    # Go tests plus web typecheck
```

## How it works

A ticker wakes every 10 seconds, reloads job files, and fires due jobs with a single-writer lease, so overlaps skip instead of stacking. The runner executes `opencode run` with the job prompt, streams output to a per-run log, and refreshes the job STATUS.md on completion.

Persistence is split by purpose. Markdown under `data/memory` holds live state the next run reads. SQLite holds append-only run history the dashboard queries. Job definitions and prompts are plain files, safe to version.

Headless runs cannot approve anything, and an `ask` permission hangs forever waiting for input. `data/opencode.json` pins permissions for runs: tools inside the data dir are allowed, while `external_directory`, `doom_loop`, and `question` are denied so approval needs fail fast. Job load warns when a prompt references absolute paths or parent traversal.

## Configuration

Copy `.env.example` to `.env` at the repo root for local dev. Only `DATA_DIR`, `PORT`, `HOST`, and `API_TOKEN` load. Precedence is make flags, then `.env`, then defaults. Never commit `.env`.

For a shipped binary, `cd` to the directory that should hold `data` and run it from the terminal, or pass `--data` with an absolute path. Double-clicking the binary on macOS starts it in your home folder, so state would land in `~/data`. To keep double-click working, copy `bin/cron-agent.command` next to the binary and open that instead. It cds to its own folder first, then execs the binary. The dashboard is embedded in the binary, so only `data` and `.env` live beside it. Place a `.env` next to the binary, or pass `--env <path>`. The binary also reads `./.env` from its working dir. Exported env vars always win.

## Docker

```sh
cp .env.example .env  # add OPENCODE_API_KEY only if you use key auth
docker compose up -d --build
```

This is the most portable way to run it. No Terminal window stays open, restarts survive reboot, and relative-path traps disappear. Model auth has two paths: either export `OPENCODE_API_KEY` (a shell export flows through Compose interpolation with no extra files, otherwise put the key in the repo `.env`, which Compose reads for every command including `down`), or skip the key entirely and run `opencode auth login` on the host, picking GitHub Copilot and completing the device flow. The auth dir bind carries that login into the container and stays writable so token refresh keeps working. Whichever path you use, the model your config selects must be served by it: a Copilot login needs a Copilot model in your global or data `opencode.json`, otherwise runs fail at exec time and the server logs a boot hint when it sees no auth at all. State lives in `./data` with the same layout as `make dev`, so jobs go in `./data/jobs` and prompts in `./data/prompts`, seeded on first boot. Do not run `make dev` and Compose against the repo at the same time, both would schedule from the same dir. Your global opencode context mounts in read-only, with the dotfiles they symlink into riding along, while the auth dir stays writable. Logs via `docker compose logs`, stop with `docker compose down`. On Linux, prefix with `UID=$(id -u) GID=$(id -g)` and uncomment the `user:` line in `compose.yaml` so the auth bind stays writable.

## Layout

- `apps/server` scheduler loop, opencode runner, SQLite store, HTTP API
- `apps/web` React dashboard, served by Go in prod and Vite in dev
- `packages/shared` API types shared by server JSON and web client
- `data` jobs, prompts, run law, permission pins, memory, db, logs
- `SPEC.md` normative behavior, `PLAN.md` build order, `docs/CONTEXT.md` decisions
