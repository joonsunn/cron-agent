package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDotEnvLoadsAllowlistedKeys(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	body := "# comment\nPORT=18091\nAPI_TOKEN=tok\nDATA_DIR=" + dir + "\nPATH=/evil\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORT", "")
	t.Setenv("API_TOKEN", "")
	t.Setenv("DATA_DIR", "")
	if got := LoadDotEnv(p); got != p {
		t.Fatalf("expected %s, got %q", p, got)
	}
	if os.Getenv("PORT") != "18091" || os.Getenv("API_TOKEN") != "tok" {
		t.Fatal("allowlisted keys not loaded")
	}
	if os.Getenv("PATH") == "/evil" {
		t.Fatal("non-allowlisted key must be ignored")
	}
}

func TestDotEnvNeverOverridesExported(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	if err := os.WriteFile(p, []byte("PORT=18092\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORT", "18093")
	LoadDotEnv(p)
	if os.Getenv("PORT") != "18093" {
		t.Fatal("exported env must win")
	}
}
