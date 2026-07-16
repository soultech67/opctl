package cmd

import (
	"testing"
	"time"

	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
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

	t.Run("day units", func(t *testing.T) {
		lower := time.Now().Add(-72 * time.Hour).Add(-time.Second)
		got, err := parseSince("3d")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		upper := time.Now().Add(-72 * time.Hour).Add(time.Second)
		if got.Before(lower) || got.After(upper) {
			t.Errorf("got %v, want ~now-72h (between %v and %v)", got, lower, upper)
		}
	})

	t.Run("day units combined with hours", func(t *testing.T) {
		lower := time.Now().Add(-36 * time.Hour).Add(-time.Second)
		got, err := parseSince("1d12h")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		upper := time.Now().Add(-36 * time.Hour).Add(time.Second)
		if got.Before(lower) || got.After(upper) {
			t.Errorf("got %v, want ~now-36h (between %v and %v)", got, lower, upper)
		}
	})

	t.Run("invalid input errors", func(t *testing.T) {
		for _, input := range []string{"not-a-time", "d", "3dxyz"} {
			if _, err := parseSince(input); err == nil {
				t.Errorf("expected an error for --since %q, got nil", input)
			}
		}
	})
}

func TestEventsCmdFlagDefaults(t *testing.T) {
	cmd := newEventsCmd(nil, &local.NodeConfig{})

	sinceFlag := cmd.Flags().Lookup("since")
	if sinceFlag == nil {
		t.Fatal("expected a --since flag")
	}
	if sinceFlag.DefValue != "" {
		t.Errorf("expected --since to default to empty (replay the entire history), got %q", sinceFlag.DefValue)
	}
	if sinceFlag.Shorthand != "t" {
		t.Errorf("expected --since shorthand -t, got %q", sinceFlag.Shorthand)
	}

	if cmd.Flags().Lookup("roots") == nil {
		t.Error("expected a --roots flag")
	}
}
