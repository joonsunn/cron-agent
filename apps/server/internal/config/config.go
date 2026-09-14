package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	DataDir string
	Port    int
	Host    string
	Token   string
}

func Resolve(dataDir string, port int, host string) Config {
	if v := os.Getenv("DATA_DIR"); v != "" && dataDir == "./data" {
		dataDir = v
	}
	if v := os.Getenv("PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			port = n
		}
	}
	if v := os.Getenv("HOST"); v != "" && host == "127.0.0.1" {
		host = v
	}
	return Config{DataDir: dataDir, Port: port, Host: host, Token: os.Getenv("API_TOKEN")}
}

func CheckOpencode(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "opencode", "--version").Run(); err != nil {
		return fmt.Errorf("opencode prerequisite missing: %w", err)
	}
	return nil
}

// AuthFilePath is where `opencode auth login` stores credentials,
// so a host Copilot login is visible without any API key in env.
func AuthFilePath() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "opencode", "auth.json")
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "opencode", "auth.json")
}

// HasModelAuth reports whether an opencode model credential is visible:
// a Zen API key, a GitHub token for the Copilot provider, or a prior
// `opencode auth login` on disk. Missing auth only warns at boot;
// runs fail at exec time with the opencode error in the run log.
func HasModelAuth() bool {
	if os.Getenv("OPENCODE_API_KEY") != "" {
		return true
	}
	if os.Getenv("GITHUB_TOKEN") != "" || os.Getenv("GH_TOKEN") != "" {
		return true
	}
	if _, err := os.Stat(AuthFilePath()); err == nil {
		return true
	}
	return false
}
