package jobs

import (
	"os"
	"path/filepath"
	"testing"
)

func writePrompt(t *testing.T, dir, name, body string) Definition {
	t.Helper()
	_ = os.MkdirAll(filepath.Join(dir, "prompts"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "prompts", name), []byte(body), 0o644)
	return Definition{Name: "t", Schedule: "* * * * *", PromptFile: "prompts/" + name, TimeoutMinutes: 5}
}

func TestPreflightCleanPrompt(t *testing.T) {
	dir := t.TempDir()
	d := writePrompt(t, dir, "a.md", "Write the current time to memory/t/now.txt")
	if w := PreflightPrompt(dir, d); w != "" {
		t.Fatalf("expected no warning, got %q", w)
	}
}

func TestPreflightAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	d := writePrompt(t, dir, "a.md", "Read /Users/foo/repos/cron-agent/AGENTS.md and summarize")
	if w := PreflightPrompt(dir, d); w == "" {
		t.Fatal("expected warning for absolute path")
	}
}

func TestPreflightTraversal(t *testing.T) {
	dir := t.TempDir()
	d := writePrompt(t, dir, "a.md", "Read ../../secrets.txt for context")
	if w := PreflightPrompt(dir, d); w == "" {
		t.Fatal("expected warning for parent traversal")
	}
}
