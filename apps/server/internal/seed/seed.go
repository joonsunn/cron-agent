package seed

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed seeddata
var templates embed.FS

// Ensure copies embedded templates into the data dir, creating it if needed.
// Existing files are never overwritten, so upgrades only add missing pieces.
func Ensure(dataDir string) error {
	return fs.WalkDir(templates, "seeddata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("seeddata", path)
		if err != nil {
			return err
		}
		dest := filepath.Join(dataDir, rel)
		if _, err := os.Stat(dest); err == nil {
			return nil
		}
		raw, err := templates.ReadFile(path)
		if err != nil {
			return fmt.Errorf("seed %s: %w", rel, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, raw, 0o644); err != nil {
			return fmt.Errorf("seed %s: %w", rel, err)
		}
		return nil
	})
}
