package runner

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"cron-agent/server/internal/jobs"
	"cron-agent/server/internal/store"
)

type Runner struct {
	DataDir string
	St      *store.Store
}

func runID() string { return time.Now().UTC().Format("20060102-150405") }

func (r *Runner) Execute(ctx context.Context, def jobs.Definition, trigger string) {
	now := time.Now().UTC()
	id := now.Format("20060102-150405.000000000")
	relPath := filepath.Join("logs", def.Name, id+".log")
	logPath := filepath.Join(r.DataDir, relPath)

	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)

	timeout := time.Duration(def.TimeoutMinutes) * time.Minute
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// DB writes use a detached context so a SIGTERM racing the run still
	// records the outcome instead of failing on a cancelled context.
	dbCtx := context.Background()
	_ = r.St.InsertRun(dbCtx, store.Run{
		ID:          id,
		Job:         def.Name,
		ScheduledAt: now.Format(time.RFC3339),
		StartedAt:   now.Format(time.RFC3339),
		FinishedAt:  sql.NullString{},
		ExitCode:    sql.NullInt64{},
		Status:      "running",
		LogPath:     relPath,
		Trigger:     trigger,
	})
	defer func() { _ = r.St.Release(context.Background(), def.Name) }()

	promptRaw, err := os.ReadFile(filepath.Join(r.DataDir, def.PromptFile))
	if err != nil {
		note := fmt.Sprintf("prompt read failed: %v", err)
		_ = os.WriteFile(logPath, []byte(note+"\n"), 0o644)
		r.finish(def.Name, id, logPath, 1, "error", note)
		return
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		r.finish(def.Name, id, logPath, 1, "error", fmt.Sprintf("log create failed: %v", err))
		return
	}
	defer logFile.Close()

	cmd := exec.CommandContext(runCtx, "opencode", "run", string(promptRaw))
	// Dir doubles as the opencode project, so data/opencode.json pins run permissions.
	cmd.Dir = r.DataDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	runErr := cmd.Run()
	status := "ok"
	code := 0
	switch {
	case runCtx.Err() == context.DeadlineExceeded:
		status = "timeout"
		code = 124
	case ctx.Err() == context.Canceled || runCtx.Err() == context.Canceled:
		// Host shutdown/restart killed the run before it could finish.
		// Recorded as interrupted so boot reconciliation and the
		// dashboard distinguish it from a command failure.
		status = "interrupted"
		code = 130
		_, _ = logFile.WriteString("\n[interrupted: host shutdown/restart]\n")
	case runErr != nil:
		status = "error"
		if ee, ok := runErr.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	r.finish(def.Name, id, logPath, code, status, "")
}

func (r *Runner) finish(job, id, logPath string, code int, status, note string) {
	ctx := context.Background()
	_ = r.St.FinishRun(ctx, id, status, code, time.Now().UTC().Format(time.RFC3339))
	r.writeStatus(job, status, code, logPath, note)
}

func (r *Runner) writeStatus(job, status string, code int, logPath, note string) {
	dir := filepath.Join(r.DataDir, "memory", job)
	_ = os.MkdirAll(dir, 0o755)
	tail := lastLines(logPath, 20)
	var b strings.Builder
	b.WriteString("# STATUS " + job + "\n\n")
	b.WriteString("Updated: " + time.Now().UTC().Format(time.RFC3339) + "\n")
	b.WriteString(fmt.Sprintf("Last: %s exit=%d\n\n", status, code))
	if note != "" {
		b.WriteString(note + "\n\n")
	}
	b.WriteString("## Tail\n\n```\n" + tail + "\n```\n")
	lines := strings.Split(b.String(), "\n")
	if len(lines) > 40 {
		lines = lines[:40]
	}
	_ = os.WriteFile(filepath.Join(dir, "STATUS.md"), []byte(strings.Join(lines, "\n")), 0o644)
}

func lastLines(path string, n int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var all []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		all = append(all, sc.Text())
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return strings.Join(all, "\n")
}

var _ = runID
