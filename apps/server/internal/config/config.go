package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
