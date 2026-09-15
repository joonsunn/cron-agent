package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type JobState struct {
	Name                string
	Enabled             bool
	Paused              bool
	LastScheduled       string
	NextRun             string
	LastStatus          string
	LastExit            sql.NullInt64
	ConsecutiveFailures int
	LockedUntil         string
}

type Run struct {
	ID          string
	Job         string
	ScheduledAt string
	StartedAt   string
	FinishedAt  sql.NullString
	ExitCode    sql.NullInt64
	Status      string
	LogPath     string
	Trigger     string
}

func Open(dataDir string) (*Store, error) {
	path := filepath.Join(dataDir, "cronagent.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS jobs (
		name TEXT PRIMARY KEY,
		enabled INTEGER NOT NULL DEFAULT 1,
		paused INTEGER NOT NULL DEFAULT 0,
		last_scheduled TEXT NOT NULL DEFAULT '',
		next_run TEXT NOT NULL DEFAULT '',
		last_status TEXT NOT NULL DEFAULT '',
		last_exit INTEGER,
		consecutive_failures INTEGER NOT NULL DEFAULT 0,
		locked_until TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		job TEXT NOT NULL,
		scheduled_at TEXT NOT NULL,
		started_at TEXT NOT NULL,
		finished_at TEXT,
		exit_code INTEGER,
		status TEXT NOT NULL,
		log_path TEXT NOT NULL,
		trigger TEXT NOT NULL
	)`)
	return err
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

func (s *Store) EnsureJob(ctx context.Context, name string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO jobs(name, enabled) VALUES(?, ?)
		ON CONFLICT(name) DO UPDATE SET enabled=excluded.enabled`, name, boolToInt(enabled))
	return err
}

func (s *Store) GetJob(ctx context.Context, name string) (JobState, error) {
	var st JobState
	var enabled, paused int
	err := s.db.QueryRowContext(ctx, `SELECT name, enabled, paused, last_scheduled, next_run,
		last_status, last_exit, consecutive_failures, locked_until FROM jobs WHERE name=?`, name).
		Scan(&st.Name, &enabled, &paused, &st.LastScheduled, &st.NextRun,
			&st.LastStatus, &st.LastExit, &st.ConsecutiveFailures, &st.LockedUntil)
	if err != nil {
		return st, err
	}
	st.Enabled = enabled == 1
	st.Paused = paused == 1
	return st, nil
}

func (s *Store) ListJobs(ctx context.Context) ([]JobState, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, enabled, paused, last_scheduled, next_run,
		last_status, last_exit, consecutive_failures, locked_until FROM jobs ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JobState
	for rows.Next() {
		var st JobState
		var enabled, paused int
		if err := rows.Scan(&st.Name, &enabled, &paused, &st.LastScheduled, &st.NextRun,
			&st.LastStatus, &st.LastExit, &st.ConsecutiveFailures, &st.LockedUntil); err != nil {
			return nil, err
		}
		st.Enabled = enabled == 1
		st.Paused = paused == 1
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) SetScheduleState(ctx context.Context, name, last, next string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET last_scheduled=?, next_run=? WHERE name=?`, last, next, name)
	return err
}

func (s *Store) Claim(ctx context.Context, name string, until time.Time) bool {
	res, err := s.db.ExecContext(ctx, `UPDATE jobs SET locked_until=?
		WHERE name=? AND (locked_until='' OR locked_until<?)`,
		until.UTC().Format(time.RFC3339), name, nowUTC())
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}

func (s *Store) Release(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET locked_until='' WHERE name=?`, name)
	return err
}

// ReconcileInterrupted marks runs left in "running" by a dead process as
// interrupted and releases all leases. Call once on boot: any lock held at
// boot belongs to a process that no longer exists (single binary, one DB),
// and any "running" row can never finish on its own because only the dead
// owner called FinishRun. Returns the number of runs marked.
func (s *Store) ReconcileInterrupted(ctx context.Context, now string) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, job FROM runs WHERE status='running'`)
	if err != nil {
		return 0, err
	}
	type stale struct{ id, job string }
	var stales []stale
	for rows.Next() {
		var st stale
		if err := rows.Scan(&st.id, &st.job); err != nil {
			_ = rows.Close()
			return 0, err
		}
		stales = append(stales, st)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, st := range stales {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE runs SET status='interrupted', exit_code=130, finished_at=? WHERE id=? AND status='running'`,
			now, st.id); err != nil {
			return len(stales), err
		}
		var cur int
		_ = s.db.QueryRowContext(ctx, `SELECT consecutive_failures FROM jobs WHERE name=?`, st.job).Scan(&cur)
		_, _ = s.db.ExecContext(ctx,
			`UPDATE jobs SET last_status='interrupted', last_exit=130, consecutive_failures=? WHERE name=?`,
			cur+1, st.job)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE jobs SET locked_until='' WHERE locked_until!=''`); err != nil {
		return len(stales), err
	}
	return len(stales), nil
}

func (s *Store) Locked(ctx context.Context, name string) bool {
	var until string
	if err := s.db.QueryRowContext(ctx, `SELECT locked_until FROM jobs WHERE name=?`, name).Scan(&until); err != nil {
		return false
	}
	if until == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, until)
	if err != nil {
		return false
	}
	return time.Now().UTC().Before(t)
}

func (s *Store) SetPaused(ctx context.Context, name string, paused bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET paused=? WHERE name=?`, boolToInt(paused), name)
	return err
}

func (s *Store) InsertRun(ctx context.Context, r Run) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO runs(id, job, scheduled_at, started_at, finished_at,
		exit_code, status, log_path, trigger) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Job, r.ScheduledAt, r.StartedAt, r.FinishedAt, r.ExitCode, r.Status, r.LogPath, r.Trigger)
	return err
}

func (s *Store) FinishRun(ctx context.Context, id, status string, exitCode int, finishedAt string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE runs SET status=?, exit_code=?, finished_at=? WHERE id=?`,
		status, exitCode, finishedAt, id)
	if err != nil {
		return err
	}
	var job string
	if err := s.db.QueryRowContext(ctx, `SELECT job FROM runs WHERE id=?`, id).Scan(&job); err != nil {
		return err
	}
	fail := 0
	if status != "ok" {
		var cur int
		_ = s.db.QueryRowContext(ctx, `SELECT consecutive_failures FROM jobs WHERE name=?`, job).Scan(&cur)
		fail = cur + 1
	}
	_, err = s.db.ExecContext(ctx, `UPDATE jobs SET last_status=?, last_exit=?, consecutive_failures=? WHERE name=?`,
		status, exitCode, fail, job)
	return err
}

func (s *Store) RecentRuns(ctx context.Context, job string, limit int) ([]Run, error) {
	var rows *sql.Rows
	var err error
	if job == "" {
		rows, err = s.db.QueryContext(ctx, `SELECT id, job, scheduled_at, started_at, finished_at,
			exit_code, status, log_path, trigger FROM runs ORDER BY started_at DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `SELECT id, job, scheduled_at, started_at, finished_at,
			exit_code, status, log_path, trigger FROM runs WHERE job=? ORDER BY started_at DESC LIMIT ?`, job, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.ID, &r.Job, &r.ScheduledAt, &r.StartedAt, &r.FinishedAt,
			&r.ExitCode, &r.Status, &r.LogPath, &r.Trigger); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Close() error { return s.db.Close() }

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
