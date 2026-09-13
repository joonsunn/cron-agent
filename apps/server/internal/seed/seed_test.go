package seed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSeedsFreshDirAndPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"jobs/example.yaml", "prompts/example.md", "opencode.json", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing seeded file %s: %v", rel, err)
		}
	}
	marker := []byte("user edited")
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), marker, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "opencode.json"))
	if string(got) != string(marker) {
		t.Fatal("Ensure overwrote an existing file")
	}
}
