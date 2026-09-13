# Data dir

Mutable state lives here. Never write state next to code.

- `jobs/*.yaml` job definitions, safe to version.
- `prompts/*.md` agent prompts referenced by jobs.
- `memory/<job>/STATUS.md` live markdown per job.
- `cronagent.db` SQLite runtime, gitignored.
- `logs/<job>/<runID>.log` raw output, gitignored.

Copy `jobs/example.yaml` to a new file and set `enabled: true` to schedule real work.
