package api

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cron-agent/server/internal/store"
)

func seedRun(t *testing.T, st *store.Store, id, job, logPath string) {
	t.Helper()
	if err := st.InsertRun(t.Context(), store.Run{
		ID:          id,
		Job:         job,
		ScheduledAt: time.Now().UTC().Format(time.RFC3339),
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
		FinishedAt:  sql.NullString{},
		ExitCode:    sql.NullInt64{},
		Status:      "ok",
		LogPath:     logPath,
		Trigger:     "test",
	}); err != nil {
		t.Fatal(err)
	}
}

func getLog(t *testing.T, s *Server, id string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/runs/"+id+"/log", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body)
}

func TestRunLogServesRelativeAndLegacyAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := &Server{St: st, DataDir: dir}

	relID := "20240101-000000.000000001"
	relBody := "relative log body"
	if err := os.MkdirAll(filepath.Join(dir, "logs", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logs", "demo", relID+".log"), []byte(relBody), 0o644); err != nil {
		t.Fatal(err)
	}
	seedRun(t, st, relID, "demo", filepath.Join("logs", "demo", relID+".log"))

	legacyID := "20240101-000000.000000002"
	legacyBody := "legacy log body"
	if err := os.WriteFile(filepath.Join(dir, "logs", "demo", legacyID+".log"), []byte(legacyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	seedRun(t, st, legacyID, "demo", filepath.Join("/Users/someone/else/data/logs/demo", legacyID+".log"))

	if code, body := getLog(t, s, relID); code != http.StatusOK || !strings.Contains(body, relBody) {
		t.Fatalf("relative log = %d %q, want 200 with body", code, body)
	}
	if code, body := getLog(t, s, legacyID); code != http.StatusOK || !strings.Contains(body, legacyBody) {
		t.Fatalf("legacy absolute log = %d %q, want 200 with body", code, body)
	}
	if code, _ := getLog(t, s, "no-such-run"); code != http.StatusNotFound {
		t.Fatalf("unknown run = %d, want 404", code)
	}
}
