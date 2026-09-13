package scheduler

import (
	"context"
	"log"
	"path/filepath"
	"time"

	"cron-agent/server/internal/jobs"
	"cron-agent/server/internal/runner"
	"cron-agent/server/internal/store"
)

type Scheduler struct {
	DataDir string
	St      *store.Store
	R       *runner.Runner
}

func New(dataDir string, st *store.Store, r *runner.Runner) *Scheduler {
	return &Scheduler{DataDir: dataDir, St: st, R: r}
}

func (s *Scheduler) Start(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	jobsDir := filepath.Join(s.DataDir, "jobs")
	defs, warnings := jobs.Load(jobsDir, s.DataDir)
	for _, w := range warnings {
		log.Printf("jobs: %s", w)
	}
	now := time.Now().UTC()
	for name, def := range defs {
		if err := s.St.EnsureJob(ctx, name, def.Enabled); err != nil {
			continue
		}
		st, err := s.St.GetJob(ctx, name)
		if err != nil {
			continue
		}
		if !def.Enabled || st.Paused {
			continue
		}
		lastStr, nextStr, fire := decide(def, st.LastScheduled, now)
		if nextStr == "" {
			continue
		}
		_ = s.St.SetScheduleState(ctx, name, lastStr, nextStr)
		if !fire {
			continue
		}
		if s.St.Locked(ctx, name) {
			log.Printf("job %s skipped: overlap", name)
			continue
		}
		until := now.Add(time.Duration(def.TimeoutMinutes) * time.Minute)
		if !s.St.Claim(ctx, name, until) {
			continue
		}
		go s.R.Execute(context.Background(), def, "schedule")
	}
}

// decide is pure scheduling logic so tests can pin it down.
// It returns the persisted last and next timestamps plus whether to fire.
// First sight seeds from now without firing, so a new job never bursts.
func decide(def jobs.Definition, lastScheduled string, now time.Time) (string, string, bool) {
	if lastScheduled == "" {
		next, err := jobs.NextRun(def, now)
		if err != nil {
			return "", "", false
		}
		return now.Format(time.RFC3339), next.Format(time.RFC3339), false
	}
	last, err := time.Parse(time.RFC3339, lastScheduled)
	if err != nil {
		last = now
	}
	next, err := jobs.NextRun(def, last)
	if err != nil {
		return "", "", false
	}
	if now.Before(next) {
		return last.Format(time.RFC3339), next.Format(time.RFC3339), false
	}
	return now.Format(time.RFC3339), next.Format(time.RFC3339), true
}

func (s *Scheduler) Trigger(ctx context.Context, name string) error {
	jobsDir := filepath.Join(s.DataDir, "jobs")
	defs, _ := jobs.Load(jobsDir, s.DataDir)
	def, ok := defs[name]
	if !ok {
		return errNotFound(name)
	}
	if s.St.Locked(ctx, name) {
		return errBusy(name)
	}
	if !s.St.Claim(ctx, name, time.Now().UTC().Add(time.Duration(def.TimeoutMinutes)*time.Minute)) {
		return errBusy(name)
	}
	go s.R.Execute(context.Background(), def, "manual")
	return nil
}

func (s *Scheduler) SetPaused(ctx context.Context, name string, paused bool) error {
	return s.St.SetPaused(ctx, name, paused)
}

type schedErr string

func (e schedErr) Error() string { return string(e) }

func errNotFound(name string) error { return schedErr("job not found: " + name) }
func errBusy(name string) error     { return schedErr("job busy: " + name) }
