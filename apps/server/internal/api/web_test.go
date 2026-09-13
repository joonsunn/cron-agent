package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestServeWebEmbedded(t *testing.T) {
	web := fstest.MapFS{
		"index.html":    {Data: []byte("<html>dashboard</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	s := &Server{WebFS: web}

	for _, tc := range []struct {
		path string
		body string
	}{
		{"/", "<html>dashboard</html>"},
		{"/assets/app.js", "console.log(1)"},
		{"/nope", "<html>dashboard</html>"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		s.Routes().ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", tc.path, res.StatusCode)
		}
		got, _ := io.ReadAll(res.Body)
		if !strings.Contains(string(got), tc.body) {
			t.Fatalf("GET %s body missing %q", tc.path, tc.body)
		}
	}
}

func TestServeWebAbsent(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET / without web assets = %d, want 404", rec.Code)
	}
}
