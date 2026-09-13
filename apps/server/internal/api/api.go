package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cron-agent/server/internal/jobs"
	"cron-agent/server/internal/scheduler"
	"cron-agent/server/internal/store"
)

type Server struct {
	St      *store.Store
	Sched   *scheduler.Scheduler
	DataDir string
	Token   string
	WebDir  string
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/jobs", s.listJobs)
	mux.HandleFunc("/api/runs", s.listRuns)
	mux.HandleFunc("/api/runs/", s.runLog)
	mux.HandleFunc("/api/jobs/", s.jobAction)
	mux.HandleFunc("/", s.serveWeb)
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defs, _ := jobs.Load(filepath.Join(s.DataDir, "jobs"), s.DataDir)
	states, _ := s.St.ListJobs(ctx)
	byName := map[string]store.JobState{}
	for _, st := range states {
		byName[st.Name] = st
	}
	type jobView struct {
		Name                string  `json:"name"`
		Schedule            string  `json:"schedule"`
		Enabled             bool    `json:"enabled"`
		NextRun             *string `json:"nextRun"`
		LastStatus          *string `json:"lastStatus"`
		LastExit            *int64  `json:"lastExit"`
		ConsecutiveFailures int     `json:"consecutiveFailures"`
	}
	out := []jobView{}
	for name, def := range defs {
		st := byName[name]
		enabled := def.Enabled && !st.Paused
		var next, last *string
		if st.NextRun != "" {
			v := st.NextRun
			next = &v
		}
		if st.LastStatus != "" {
			v := st.LastStatus
			last = &v
		}
		var exit *int64
		if st.LastExit.Valid {
			v := st.LastExit.Int64
			exit = &v
		}
		out = append(out, jobView{
			Name: name, Schedule: def.Schedule, Enabled: enabled,
			NextRun: next, LastStatus: last, LastExit: exit,
			ConsecutiveFailures: st.ConsecutiveFailures,
		})
	}
	writeJSON(w, out)
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	job := q.Get("job")
	limit := 50
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	runs, err := s.St.RecentRuns(r.Context(), job, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	type runView struct {
		ID          string  `json:"id"`
		Job         string  `json:"job"`
		ScheduledAt string  `json:"scheduledAt"`
		StartedAt   string  `json:"startedAt"`
		FinishedAt  *string `json:"finishedAt"`
		ExitCode    *int64  `json:"exitCode"`
		Status      string  `json:"status"`
		Trigger     string  `json:"trigger"`
	}
	out := make([]runView, 0, len(runs))
	for _, rn := range runs {
		var fin *string
		if rn.FinishedAt.Valid {
			v := rn.FinishedAt.String
			fin = &v
		}
		var code *int64
		if rn.ExitCode.Valid {
			v := rn.ExitCode.Int64
			code = &v
		}
		out = append(out, runView{ID: rn.ID, Job: rn.Job, ScheduledAt: rn.ScheduledAt,
			StartedAt: rn.StartedAt, FinishedAt: fin, ExitCode: code, Status: rn.Status, Trigger: rn.Trigger})
	}
	writeJSON(w, out)
}

func (s *Server) runLog(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	id = strings.TrimSuffix(id, "/log")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	runs, err := s.St.RecentRuns(r.Context(), "", 200)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	for _, rn := range runs {
		if rn.ID == id {
			http.ServeFile(w, r, rn.LogPath)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) jobAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if s.Token != "" {
		if r.Header.Get("Authorization") != "Bearer "+s.Token {
			http.Error(w, "unauthorized", 401)
			return
		}
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	name, action := parts[0], parts[1]
	switch action {
	case "trigger":
		if err := s.Sched.Trigger(r.Context(), name); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case "pause":
		if err := s.Sched.SetPaused(r.Context(), name, true); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case "resume":
		if err := s.Sched.SetPaused(r.Context(), name, false); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) serveWeb(w http.ResponseWriter, r *http.Request) {
	if s.WebDir == "" {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.WebDir, strings.TrimPrefix(r.URL.Path, "/"))
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.WebDir, "index.html"))
}
