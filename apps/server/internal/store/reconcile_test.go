package store

import (
	"context"
	"testing"
	"time"
)

func TestReconcileInterruptedMarksRunningAndReleasesLease(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	if err := st.EnsureJob(ctx, "daily", true); err != nil {
		t.Fatal(err)
	}
	if !st.Claim(ctx, "daily", time.Now().UTC().Add(5*time.Minute)) {
		t.Fatal("claim failed")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := st.InsertRun(ctx, Run{
		ID: "r1", Job: "daily", ScheduledAt: now, StartedAt: now,
		Status: "running", LogPath: "logs/daily/r1.log", Trigger: "schedule",
	}); err != nil {
		t.Fatal(err)
	}

	n, err := st.ReconcileInterrupted(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("reconciled = %d, want 1", n)
	}
	runs, err := st.RecentRuns(ctx, "daily", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status != "interrupted" {
		t.Fatalf("run status = %+v, want interrupted", runs)
	}
	if !runs[0].FinishedAt.Valid || !runs[0].ExitCode.Valid || runs[0].ExitCode.Int64 != 130 {
		t.Fatalf("run not closed properly: %+v", runs[0])
	}
	if st.Locked(ctx, "daily") {
		t.Fatal("lease still held after reconcile")
	}
	js, err := st.GetJob(ctx, "daily")
	if err != nil {
		t.Fatal(err)
	}
	if js.LastStatus != "interrupted" || js.ConsecutiveFailures != 1 {
		t.Fatalf("job state = %+v, want interrupted/1 failure", js)
	}
}

func TestReconcileInterruptedEmptyIsNoop(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	n, err := st.ReconcileInterrupted(context.Background(), time.Now().UTC().Format(time.RFC3339))
	if err != nil || n != 0 {
		t.Fatalf("empty reconcile = %d, %v; want 0, nil", n, err)
	}
}
