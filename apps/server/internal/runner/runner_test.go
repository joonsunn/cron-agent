package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cron-agent/server/internal/jobs"
	"cron-agent/server/internal/store"
)

func TestExecuteMissingPromptStillWritesLog(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	r := &Runner{DataDir: dir, St: st}
	r.Execute(t.Context(), jobs.Definition{
		Name:           "broken",
		Schedule:       "0 9 * * *",
		PromptFile:     "prompts/missing.md",
		Timezone:       "UTC",
		TimeoutMinutes: 5,
	}, "test")

	matches, err := filepath.Glob(filepath.Join(dir, "logs", "broken", "*.log"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one log file, got %v, %v", matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "prompt read failed") {
		t.Fatalf("log missing failure note: %q", raw)
	}
}
