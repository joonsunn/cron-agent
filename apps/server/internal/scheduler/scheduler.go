package scheduler

import (
	"context"
	"log"
	"path/filepath"
	"sync"
	"time"

	"cron-agent/server/internal/jobs"
	"cron-agent/server/internal/runner"
	"cron-agent/server/internal/store"
)

// DefaultMaxConcurrent caps how many agent runs may execute at once.
// Excess firings wait in an in-memory FIFO queue instead of starting
// new processes, so a burst of due jobs cannot overload the host.
const DefaultMaxConcurrent = 3

type pendingJob struct {
	def     jobs.Definition
	trigger string
}

type Scheduler struct {
	DataDir string
	St      *store.Store
	R       *runner.Runner

	// MaxConcurrent bounds simultaneous runs. Values <= 0 mean the default.
	MaxConcurrent int

	// execFn runs a claimed job. Tests override it; production uses Runner.Execute.
	execFn func(def jobs.Definition, trigger string)

	mu      sync.Mutex
	running int
	pending []pendingJob
	queued  map[string]bool
}

func New(dataDir string, st *store.Store, r *runner.Runner) *Scheduler {
	return &Scheduler{DataDir: dataDir, St: st, R: r, MaxConcurrent: DefaultMaxConcurrent, queued: map[string]bool{}}
}

func (s *Scheduler) limit() int {
	if s.MaxConcurrent <= 0 {
		return DefaultMaxConcurrent
	}
	return s.MaxConcurrent
}

// Running returns the number of currently executing runs.
func (s *Scheduler) Running() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// PendingDepth returns the number of queued runs waiting for a slot.
func (s *Scheduler) PendingDepth() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
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
		if _, err := s.dispatch(ctx, def, "schedule"); err != nil {
			log.Printf("job %s skipped: overlap", name)
			continue
		}
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
	_, err := s.dispatch(ctx, def, "manual")
	return err
}

// dispatch claims a run slot for one firing. It starts the job when a slot
// is free and queues it FIFO when at capacity. At most one pending entry per
// job name is kept, so a job that fires again while queued or running reports
// busy instead of stacking duplicates.
func (s *Scheduler) dispatch(ctx context.Context, def jobs.Definition, trigger string) (string, error) {
	if s.St.Locked(ctx, def.Name) {
		return "", errBusy(def.Name)
	}
	s.mu.Lock()
	if s.queued[def.Name] {
		s.mu.Unlock()
		return "", errBusy(def.Name)
	}
	if s.running >= s.limit() {
		s.pending = append(s.pending, pendingJob{def: def, trigger: trigger})
		s.queued[def.Name] = true
		running, depth := s.running, len(s.pending)
		s.mu.Unlock()
		log.Printf("job %s queued: %d running, depth %d", def.Name, running, depth)
		return "queued", nil
	}
	s.running++
	s.mu.Unlock()
	until := time.Now().UTC().Add(time.Duration(def.TimeoutMinutes) * time.Minute)
	if !s.St.Claim(ctx, def.Name, until) {
		s.abortSlot()
		return "", errBusy(def.Name)
	}
	s.goExecute(def, trigger)
	return "started", nil
}

func (s *Scheduler) goExecute(def jobs.Definition, trigger string) {
	exec := s.execFn
	if exec == nil && s.R != nil {
		r := s.R
		exec = func(d jobs.Definition, t string) { r.Execute(context.Background(), d, t) }
	}
	if exec == nil {
		s.abortSlot()
		return
	}
	go func() {
		exec(def, trigger)
		s.onRunDone()
	}()
}

// abortSlot releases a slot acquired before a failed claim, then starts the
// next queued job when one is waiting.
func (s *Scheduler) abortSlot() {
	s.mu.Lock()
	s.running--
	s.mu.Unlock()
	s.pump()
}

// onRunDone frees the finished run's slot and starts the next queued job.
func (s *Scheduler) onRunDone() {
	s.mu.Lock()
	s.running--
	s.mu.Unlock()
	s.pump()
}

// pump starts queued jobs FIFO while slots are free. It claims the DB lease
// at start time so the lease covers the run, not the queue wait. Entries for
// paused jobs are dropped and their leases released.
func (s *Scheduler) pump() {
	for {
		s.mu.Lock()
		if s.running >= s.limit() || len(s.pending) == 0 {
			s.mu.Unlock()
			return
		}
		next := s.pending[0]
		s.pending = s.pending[1:]
		delete(s.queued, next.def.Name)
		s.running++
		s.mu.Unlock()
		if st, err := s.St.GetJob(context.Background(), next.def.Name); err == nil && st.Paused {
			_ = s.St.Release(context.Background(), next.def.Name)
			s.mu.Lock()
			s.running--
			s.mu.Unlock()
			continue
		}
		until := time.Now().UTC().Add(time.Duration(next.def.TimeoutMinutes) * time.Minute)
		if !s.St.Claim(context.Background(), next.def.Name, until) {
			s.mu.Lock()
			s.running--
			s.mu.Unlock()
			continue
		}
		s.goExecute(next.def, next.trigger)
		return
	}
}

func (s *Scheduler) SetPaused(ctx context.Context, name string, paused bool) error {
	return s.St.SetPaused(ctx, name, paused)
}

type schedErr string

func (e schedErr) Error() string { return string(e) }

func errNotFound(name string) error { return schedErr("job not found: " + name) }
func errBusy(name string) error     { return schedErr("job busy: " + name) }
