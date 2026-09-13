# AGENTS.md

Repo-local rules for cron-agent.

## Commands

Use the root Makefile only. Do not call `go` or `pnpm` directly except to debug a failing target.

- `make setup` installs server deps and web deps.
- `make dev` runs Go API and Vite dev in parallel.
- `make build` builds web then server with static serving.
- `make run` serves production build against `./data`.
- `make test` runs Go tests plus web typecheck.

## Boundaries

- Server owns `apps/server` and SQLite. Web never touches the db.
- Mutable state lives only under the data dir. Never write state next to code.
- Job definitions are data, not code. Validate on load, fail open with a warning and skip the bad file.
- Never commit `data/*.db`, `data/logs`, or secrets. Example job stays disabled.

## Style

- Go: stdlib first, small packages, context deadlines on every exec.
- Web: TypeScript strict, escape all server output before render.
- One logical line per paragraph or list item in markdown docs.
