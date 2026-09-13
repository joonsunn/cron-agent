package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

type Definition struct {
	Name            string `yaml:"name"`
	Schedule        string `yaml:"schedule"`
	PromptFile      string `yaml:"promptFile"`
	Timezone        string `yaml:"timezone"`
	TimeoutMinutes  int    `yaml:"timeoutMinutes"`
	Enabled         bool   `yaml:"enabled"`
	NotifyOnFailure string `yaml:"notifyOnFailure"`
}

var nameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

func Load(jobsDir, dataDir string) (map[string]Definition, []string) {
	defs := map[string]Definition{}
	var warnings []string
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		return defs, []string{fmt.Sprintf("jobs dir unreadable: %v", err)}
	}
	for _, e := range entries {
		if e.IsDir() || !(strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(jobsDir, e.Name()))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: unreadable: %v", e.Name(), err))
			continue
		}
		var d Definition
		if err := yaml.Unmarshal(raw, &d); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: bad yaml: %v", e.Name(), err))
			continue
		}
		if err := Validate(d, dataDir); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: skipped: %v", e.Name(), err))
			continue
		}
		if w := PreflightPrompt(dataDir, d); w != "" {
			warnings = append(warnings, fmt.Sprintf("%s: %s", e.Name(), w))
		}
		defs[d.Name] = d
	}
	return defs, warnings
}

func Validate(d Definition, dataDir string) error {
	if !nameRe.MatchString(d.Name) {
		return fmt.Errorf("bad name %q, use lowercase letters, digits, dashes", d.Name)
	}
	if _, err := cron.ParseStandard(d.Schedule); err != nil {
		return fmt.Errorf("bad schedule %q: %w", d.Schedule, err)
	}
	if d.TimeoutMinutes < 1 || d.TimeoutMinutes > 120 {
		return fmt.Errorf("timeoutMinutes must be 1..120")
	}
	if d.PromptFile == "" {
		return fmt.Errorf("promptFile is required")
	}
	tz := d.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("bad timezone %q: %w", tz, err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, d.PromptFile)); err != nil {
		return fmt.Errorf("prompt file missing: %s", d.PromptFile)
	}
	return nil
}

func NextRun(d Definition, from time.Time) (time.Time, error) {
	tz := d.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}
	sched, err := cron.ParseStandard(d.Schedule)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from.In(loc)), nil
}
