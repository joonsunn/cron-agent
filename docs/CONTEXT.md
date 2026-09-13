# Context and memory

## Decisions

- Markdown for live memory, SQLite for history. Markdown is what opencode reads next. SQLite answers what ran, when, and with what exit. See SPEC.md persistence.
- No OS scheduler. One Go process owns the tick loop so start and kill move together.
- `opencode` is a hard prerequisite, checked at boot with `opencode --version`.
- `data/opencode.json` pins opencode permissions for runs. Tools inside the data dir are allowed, `external_directory`, `doom_loop`, and `question` are denied so approval needs fail fast instead of hanging a headless run. Job load warns when a prompt references absolute paths or parent traversal.
- Cron parsing with `github.com/robfig/cron/v3`. SQLite driver is `modernc.org/sqlite` (pure Go, no cgo) to keep `make build` trivial on mac and linux.
- Data dir lives at repo root `./data` for dev, outside `apps/server`. Override with `--data` or `DATA_DIR`. Never store state beside the binary.
- Web ships embedded in the Go binary in prod. Vite dev server only for local UI work.

## Data dir contract

- `data/jobs/*.yaml` job definitions, seeded with a disabled example on fresh boot.
- `data/prompts/*.md` agent prompts referenced by jobs, seeded with an example on fresh boot.
- `data/opencode.json` permission pins for runs, seeded on fresh boot and gitignored.
- `data/AGENTS.md` run law for scheduled agents, seeded on fresh boot and gitignored. Nearest AGENTS.md wins, so runs see this instead of the repo root file.
- `data/memory/<job>/STATUS.md` live markdown per job, max 40 lines.
- `data/cronagent.db` SQLite runtime, gitignored.
- `data/logs/<job>/<runID>.log` raw output, gitignored.

## Conventions

- All schedules evaluate in job timezone, default UTC.
- Ticker 10s, lease refresh 30s, dashboard poll 10s.
- Runs are insert only. Overlaps skip, never parallel run the same job.
- Job file adds and edits apply on the next 10s tick. No restart needed for job changes, restart only for server code changes.
- Example job ships with `enabled: false` so setup never spends model calls.
- Env file search is `--env` path, then `./.env`, then `.env` next to the binary. Only `DATA_DIR`, `PORT`, `HOST`, `API_TOKEN` load, exported env always wins.
- Fresh data dirs seed from templates embedded in the binary. Seed sources live in `apps/server/internal/seed/seeddata`, the source of truth. Runtime files under `data/` are gitignored except `data/README.md`.

## Open items

- Token auth middleware is stubbed until first exposed deploy.
- Log tail streaming deferred, dashboard polls full log in v1.
- Backfill policy is warn only, no silent catch-up runs.
