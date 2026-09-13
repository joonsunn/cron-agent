package config

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// allowlist keeps dotenv loading scoped to cron-agent settings.
// Anything else in the file is ignored so a stray PATH line cannot
// rewire the process the server shells opencode from.
var envAllow = map[string]bool{
	"DATA_DIR":  true,
	"PORT":      true,
	"API_TOKEN": true,
}

// LoadDotEnv loads the first dotenv file found and returns its path.
// Search order is explicit path, ./.env in cwd, .env next to the binary.
// Exported env vars always win, loaded values never override them.
func LoadDotEnv(explicit string) string {
	cands := []string{}
	if explicit != "" {
		cands = append(cands, explicit)
	}
	cands = append(cands, ".env")
	if exe, err := os.Executable(); err == nil {
		cands = append(cands, filepath.Join(filepath.Dir(exe), ".env"))
	}
	for _, p := range cands {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			if parseDotEnv(p) {
				return p
			}
			return ""
		}
	}
	return ""
}

func parseDotEnv(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	ok := true
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if !found || !envKeyRe.MatchString(key) || !envAllow[key] {
			continue
		}
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') ||
			(val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return ok && sc.Err() == nil
}
