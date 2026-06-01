package cmd

import (
	"testing"
	"time"
)

func TestParseSince(t *testing.T) {
	t.Run("RFC3339 timestamp", func(t *testing.T) {
		want := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
		got, err := parseSince("2026-05-31T12:00:00Z")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("duration relative to now", func(t *testing.T) {
		lower := time.Now().Add(-2 * time.Hour).Add(-time.Second)
		got, err := parseSince("2h")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		upper := time.Now().Add(-2 * time.Hour).Add(time.Second)
		if got.Before(lower) || got.After(upper) {
			t.Errorf("got %v, want ~now-2h (between %v and %v)", got, lower, upper)
		}
	})

	t.Run("invalid input errors", func(t *testing.T) {
		if _, err := parseSince("not-a-time"); err == nil {
			t.Error("expected an error for invalid --since, got nil")
		}
	})
}
