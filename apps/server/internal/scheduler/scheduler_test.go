package scheduler

import (
	"testing"
	"time"

	"cron-agent/server/internal/jobs"
)

func everyMinute() jobs.Definition {
	return jobs.Definition{Name: "t", Schedule: "* * * * *", Timezone: "UTC", TimeoutMinutes: 5}
}

func TestDecideSeedsWithoutFiring(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 30, 0, time.UTC)
	last, next, fire := decide(everyMinute(), "", now)
	if fire {
		t.Fatal("first sight must not fire")
	}
	if last == "" || next == "" || last == "seed" {
		t.Fatalf("bad seed state last=%q next=%q", last, next)
	}
}

func TestDecideFiresOnceAtBoundary(t *testing.T) {
	def := everyMinute()
	boundary := time.Date(2026, 9, 13, 12, 1, 0, 0, time.UTC)
	_, _, fire := decide(def, "2026-09-13T12:00:00Z", boundary)
	if !fire {
		t.Fatal("expected fire at boundary")
	}
	after := time.Date(2026, 9, 13, 12, 1, 10, 0, time.UTC)
	_, _, refire := decide(def, "2026-09-13T12:01:00Z", after)
	if refire {
		t.Fatal("must not refire right after firing")
	}
}

func TestDecideSkipsWhenNotDue(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 30, 0, time.UTC)
	_, _, fire := decide(everyMinute(), "2026-09-13T12:00:00Z", now)
	if fire {
		t.Fatal("must not fire before next boundary")
	}
}
