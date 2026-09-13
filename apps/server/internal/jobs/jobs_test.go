package jobs_test

import (
	"os"
	"path/filepath"
	"testing"

	"cron-agent/server/internal/jobs"
)

func TestValidateRejectsBadSchedule(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "prompts"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "prompts", "a.md"), []byte("hi"), 0o644)
	d := jobs.Definition{Name: "ok", Schedule: "not-a-cron", PromptFile: "prompts/a.md", TimeoutMinutes: 5}
	if err := jobs.Validate(d, dir); err == nil {
		t.Fatal("expected error")
	}
}
