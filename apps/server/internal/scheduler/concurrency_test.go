package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"cron-agent/server/internal/jobs"
	"cron-agent/server/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func testDef(name string) jobs.Definition {
	return jobs.Definition{Name: name, Schedule: "* * * * *", Timezone: "UTC", TimeoutMinutes: 5}
}

func ensureJobs(t *testing.T, st *store.Store, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := st.EnsureJob(context.Background(), n, true); err != nil {
			t.Fatal(err)
		}
	}
}

// blockingExec records start order and blocks each run until released.
type blockingExec struct {
	mu      sync.Mutex
	starts  []string
	unblock map[string]chan struct{}
	started map[string]chan struct{}
}

func newBlockingExec() *blockingExec {
	return &blockingExec{unblock: map[string]chan struct{}{}, started: map[string]chan struct{}{}}
}

func (b *blockingExec) fn(def jobs.Definition, _ string) {
	b.mu.Lock()
	b.starts = append(b.starts, def.Name)
	ch, ok := b.unblock[def.Name]
	if !ok {
		ch = make(chan struct{})
		b.unblock[def.Name] = ch
	}
	sig, ok := b.started[def.Name]
	if !ok {
		sig = make(chan struct{})
		b.started[def.Name] = sig
	}
	b.mu.Unlock()
	close(sig)
	<-ch
}

func (b *blockingExec) waitStarted(t *testing.T, name string) {
	t.Helper()
	b.mu.Lock()
	sig, ok := b.started[name]
	if !ok {
		sig = make(chan struct{})
		b.started[name] = sig
	}
	b.mu.Unlock()
	select {
	case <-sig:
	case <-time.After(2 * time.Second):
		t.Fatalf("job %s never started", name)
	}
}

func (b *blockingExec) release(name string) {
	b.mu.Lock()
	ch, ok := b.unblock[name]
	if !ok {
		ch = make(chan struct{})
		b.unblock[name] = ch
	}
	b.mu.Unlock()
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestDispatchQueuesBeyondLimitFIFO(t *testing.T) {
	st := testStore(t)
	ensureJobs(t, st, "a", "b", "c")
	ex := newBlockingExec()
	s := New("", st, nil)
	s.MaxConcurrent = 2
	s.execFn = ex.fn
	ctx := context.Background()

	if _, err := s.dispatch(ctx, testDef("a"), "schedule"); err != nil {
		t.Fatalf("dispatch a: %v", err)
	}
	if _, err := s.dispatch(ctx, testDef("b"), "schedule"); err != nil {
		t.Fatalf("dispatch b: %v", err)
	}
	ex.waitStarted(t, "a")
	ex.waitStarted(t, "b")

	got, err := s.dispatch(ctx, testDef("c"), "schedule")
	if err != nil {
		t.Fatalf("dispatch c: %v", err)
	}
	if got != "queued" {
		t.Fatalf("dispatch c = %q, want queued", got)
	}
	if n := s.PendingDepth(); n != 1 {
		t.Fatalf("pending depth = %d, want 1", n)
	}
	if n := s.Running(); n != 2 {
		t.Fatalf("running = %d, want 2", n)
	}

	ex.release("a")
	ex.waitStarted(t, "c")
	waitFor(t, "queue drain", func() bool { return s.PendingDepth() == 0 })
	ex.release("b")
	ex.release("c")
	waitFor(t, "all done", func() bool { return s.Running() == 0 })

	ex.mu.Lock()
	defer ex.mu.Unlock()
	if len(ex.starts) != 3 || ex.starts[0] != "a" || ex.starts[1] != "b" || ex.starts[2] != "c" {
		t.Fatalf("start order = %v, want [a b c]", ex.starts)
	}
}

func TestDispatchCoalescesDuplicateWhileQueued(t *testing.T) {
	st := testStore(t)
	ensureJobs(t, st, "x", "y")
	ex := newBlockingExec()
	s := New("", st, nil)
	s.MaxConcurrent = 1
	s.execFn = ex.fn
	ctx := context.Background()

	if _, err := s.dispatch(ctx, testDef("x"), "schedule"); err != nil {
		t.Fatal(err)
	}
	ex.waitStarted(t, "x")
	if _, err := s.dispatch(ctx, testDef("y"), "schedule"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.dispatch(ctx, testDef("y"), "schedule"); err == nil {
		t.Fatal("expected busy on duplicate queued job")
	}
	if n := s.PendingDepth(); n != 1 {
		t.Fatalf("pending depth = %d, want 1", n)
	}
	ex.release("x")
	ex.waitStarted(t, "y")
	ex.release("y")
	waitFor(t, "all done", func() bool { return s.Running() == 0 && s.PendingDepth() == 0 })
}

func TestTriggerQueuesWhenFull(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"alpha", "beta"} {
		yml := "name: " + n + "\nschedule: \"* * * * *\"\npromptFile: prompts/" + n + ".md\ntimezone: UTC\ntimeoutMinutes: 5\nenabled: true\n"
		if err := os.MkdirAll(filepath.Join(dir, "jobs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "jobs", n+".yaml"), []byte(yml), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "prompts", n+".md"), []byte("do "+n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ensureJobs(t, st, "alpha", "beta")
	ex := newBlockingExec()
	s := New(dir, st, nil)
	s.MaxConcurrent = 1
	s.execFn = ex.fn
	ctx := context.Background()

	if err := s.Trigger(ctx, "alpha"); err != nil {
		t.Fatalf("trigger alpha: %v", err)
	}
	ex.waitStarted(t, "alpha")
	if err := s.Trigger(ctx, "beta"); err != nil {
		t.Fatalf("trigger beta should queue, got: %v", err)
	}
	waitFor(t, "beta queued", func() bool { return s.PendingDepth() == 1 })
	if err := s.Trigger(ctx, "beta"); err == nil {
		t.Fatal("expected busy on duplicate queued trigger")
	}
	ex.release("alpha")
	ex.waitStarted(t, "beta")
	ex.release("beta")
	waitFor(t, "all done", func() bool { return s.Running() == 0 && s.PendingDepth() == 0 })
}
