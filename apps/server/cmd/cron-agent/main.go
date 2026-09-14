package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cron-agent/server/internal/api"
	"cron-agent/server/internal/config"
	"cron-agent/server/internal/runner"
	"cron-agent/server/internal/scheduler"
	"cron-agent/server/internal/seed"
	"cron-agent/server/internal/store"
)

//go:embed webdist
var webDist embed.FS

func main() {
	dataDir := flag.String("data", "./data", "data dir for jobs, memory, db, logs")
	port := flag.Int("port", 8080, "http port")
	host := flag.String("host", "127.0.0.1", "http host (use 0.0.0.0 in containers)")
	envFile := flag.String("env", "", "dotenv file (default: ./.env, then .env next to the binary)")
	flag.Parse()

	if src := config.LoadDotEnv(*envFile); src != "" {
		log.Printf("env from %s", src)
	}

	cfg := config.Resolve(*dataDir, *port, *host)

	absData, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		log.Fatalf("data dir: %v", err)
	}
	cfg.DataDir = absData

	for _, sub := range []string{"jobs", "prompts", "memory", "logs"} {
		if err := os.MkdirAll(filepath.Join(cfg.DataDir, sub), 0o755); err != nil {
			log.Fatalf("data dir: %v", err)
		}
	}

	if err := seed.Ensure(cfg.DataDir); err != nil {
		log.Fatalf("seed data dir: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := config.CheckOpencode(ctx); err != nil {
		log.Fatalf("%v", err)
	}
	if !config.HasModelAuth() {
		log.Printf("no model auth found: set OPENCODE_API_KEY or run `opencode auth login` (e.g. GitHub Copilot)")
	}

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	r := &runner.Runner{DataDir: cfg.DataDir, St: st}
	sched := scheduler.New(cfg.DataDir, st, r)
	go sched.Start(ctx)

	webFS, webDir := resolveWeb()
	srv := &api.Server{St: st, Sched: sched, DataDir: cfg.DataDir, Token: cfg.Token, WebDir: webDir, WebFS: webFS}
	httpSrv := &http.Server{Addr: addr(cfg.Host, cfg.Port), Handler: srv.Routes()}

	go func() {
		log.Printf("cron-agent on %s data=%s", httpSrv.Addr, cfg.DataDir)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)
}

func addr(host string, port int) string { return host + ":" + itoa(port) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// resolveWeb prefers the dashboard embedded at build time and falls back
// to a dist dir on disk, so `go run` keeps working without a web build.
func resolveWeb() (fs.FS, string) {
	if sub, err := fs.Sub(webDist, "webdist"); err == nil {
		if f, err := sub.Open("index.html"); err == nil {
			_ = f.Close()
			return sub, ""
		}
	}
	return nil, resolveWebDir()
}

func resolveWebDir() string {
	for _, cand := range []string{"apps/web/dist", "../web/dist", "web/dist", "./dist"} {
		if st, err := os.Stat(filepath.Join(cand, "index.html")); err == nil && !st.IsDir() {
			return cand
		}
	}
	return ""
}
