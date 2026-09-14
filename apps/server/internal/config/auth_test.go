package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasModelAuthFalseWhenNothingPresent(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if HasModelAuth() {
		t.Fatal("expected no auth with empty env and empty data home")
	}
}

func TestHasModelAuthTrueWithKey(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "test-key")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if !HasModelAuth() {
		t.Fatal("expected auth with OPENCODE_API_KEY set")
	}
}

func TestHasModelAuthTrueWithLoginOnDisk(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	dataHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataHome, "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataHome, "opencode", "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", dataHome)
	if !HasModelAuth() {
		t.Fatal("expected auth with auth.json on disk (e.g. Copilot login)")
	}
}
