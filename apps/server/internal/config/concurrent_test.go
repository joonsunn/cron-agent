package config

import "testing"

func TestEffectiveMaxConcurrent(t *testing.T) {
	t.Setenv("MAX_CONCURRENT", "")
	if got := EffectiveMaxConcurrent(0); got != DefaultMaxConcurrent {
		t.Fatalf("default = %d, want %d", got, DefaultMaxConcurrent)
	}
	if got := EffectiveMaxConcurrent(5); got != 5 {
		t.Fatalf("explicit = %d, want 5", got)
	}
	t.Setenv("MAX_CONCURRENT", "7")
	if got := EffectiveMaxConcurrent(5); got != 7 {
		t.Fatalf("env override = %d, want 7", got)
	}
	t.Setenv("MAX_CONCURRENT", "bogus")
	if got := EffectiveMaxConcurrent(0); got != DefaultMaxConcurrent {
		t.Fatalf("invalid env = %d, want default %d", got, DefaultMaxConcurrent)
	}
	t.Setenv("MAX_CONCURRENT", "0")
	if got := EffectiveMaxConcurrent(0); got != DefaultMaxConcurrent {
		t.Fatalf("zero env = %d, want default %d", got, DefaultMaxConcurrent)
	}
}
