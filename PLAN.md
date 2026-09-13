# Plan: implementation organisation

## Layout (monorepo)

- `apps/server` Go service: scheduler loop, opencode runner, SQLite store, HTTP API, static web serving.
- `apps/web` React dashboard: Vite, TypeScript, polls the Go API.
- `packages/shared` shared API shapes: TypeScript types mirroring Go JSON.
- `data` local resting place: job definitions, prompts, markdown memory, SQLite db, logs. Gitignored except examples.
- `docs` decisions and context.
- `Makefile` at root as the only entry point.

## Phase 0: docs and memory (done first)

- `SPEC.md` normative behavior.
- `PLAN.md` this file.
- `docs/CONTEXT.md` decisions that assist implementation.
- `AGENTS.md` repo-local working rules.

## Phase 1: scaffold and Makefile

- `pnpm-workspace.yaml`, root `.gitignore`, `data/.gitkeep` plus `data/README.md`.
- `apps/server/go.mod`, minimal `cmd/cron-agent/main.go` with `--data` and `--port`.
- `apps/web` Vite scaffold with typecheck script.
- `packages/shared` types only.
- Root `Makefile` with `setup`, `dev`, `build`, `run`, `test`, `clean`.

## Phase 2: server

Order: config, store schema, job loader, scheduler ticker, runner exec, API routes, static serving.

- `internal/config` resolves data dir, port, token, opencode check.
- `internal/store` opens SQLite (WAL), migrates `jobs` and `runs`, lease helpers.
- `internal/jobs` loads and validates `<data>/jobs/*.yaml`.
- `internal/scheduler` 10s tick, cron next-run, lease claim, skip on overlap.
- `internal/runner` context timeout, `opencode run`, log streaming, STATUS update.
- `internal/api` health, jobs, runs, log, trigger, pause, resume.
- `webdist` embed or filesystem fallback to `apps/web/dist`.

Tests: cron next-run cases, lease claim and expiry, job validation, API shape test with temp data dir.

## Phase 3: web

- API client against `/api/*` with shared types.
- Views: job list, run history, log view, trigger and pause buttons.
- Poll every 10s. Escape all rendered output.

## Phase 4: verify

- `make build`, `make test`, `make run` smoke against temp data dir.
- Confirm example job ships disabled so no model call happens by accident.
- Confirm dashboard loads from Go static serving.

## Ownership

Server owns scheduling truth. Web never writes schedules directly, only trigger, pause, resume. Data dir owns all mutable state. No other writer touches SQLite except the server.
