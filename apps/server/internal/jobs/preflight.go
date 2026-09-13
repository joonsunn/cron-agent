package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var absPathRe = regexp.MustCompile(`(^|[\s"'=])(/(Users|home|root|etc|var|tmp|opt|private|System|Applications)[/\s"']|~/)`)
var traversalRe = regexp.MustCompile(`\.\.(/|\\|$)`)

// PreflightPrompt scans prompt text for references that escape the data dir.
// Absolute paths and parent traversal trip external_directory, which
// data/opencode.json denies, so the agent call would fail instead of hang.
// This returns a warning string and never blocks registration.
func PreflightPrompt(dataDir string, d Definition) string {
	raw, err := os.ReadFile(filepath.Join(dataDir, d.PromptFile))
	if err != nil {
		return ""
	}
	text := string(raw)
	if absPathRe.MatchString(text) {
		return fmt.Sprintf("%s: prompt references an absolute path outside the data dir, agent access will be denied", d.Name)
	}
	if traversalRe.MatchString(text) {
		return fmt.Sprintf("%s: prompt uses parent traversal, keep all paths inside the data dir", d.Name)
	}
	return ""
}
