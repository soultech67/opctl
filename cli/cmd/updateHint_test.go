package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/spf13/cobra"
)

func TestGetUpdateHintReturnsHintForNewerRelease(t *testing.T) {
	dataDir := t.TempDir()
	currentTime := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	calledRepo := ""

	hint := getUpdateHint(updateHintConfig{
		args:           []string{"run"},
		command:        runnableCommand("run"),
		currentVersion: "0.1.77",
		dataDir:        dataDir,
		detectLatestRelease: func(repo string) (semver.Version, bool, error) {
			calledRepo = repo
			return semver.MustParse("0.1.78"), true, nil
		},
		now:            func() time.Time { return currentTime },
		selfUpdateRepo: "soultech67/opctl",
	})

	if calledRepo != "soultech67/opctl" {
		t.Fatalf("expected detector to use fork repo, got %q", calledRepo)
	}
	if hint != "opctl 0.1.78 is available. Run 'opctl self-update' to update." {
		t.Fatalf("expected update hint for 0.1.78, got %q", hint)
	}

	cache := readUpdateHintCache(dataDir)
	if !cache.LastCheckedAt.Equal(currentTime) {
		t.Fatalf("expected cache last checked at %s, got %s", currentTime, cache.LastCheckedAt)
	}
	if cache.LatestVersion != "0.1.78" {
		t.Fatalf("expected latest version cached as 0.1.78, got %q", cache.LatestVersion)
	}
}

func TestMaybePrintUpdateHintWritesToConfiguredWarningWriter(t *testing.T) {
	dataDir := t.TempDir()
	currentTime := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	warningWriter := bytes.Buffer{}

	maybePrintUpdateHint(updateHintConfig{
		args:           []string{"run"},
		command:        runnableCommand("run"),
		currentVersion: "0.1.77",
		dataDir:        dataDir,
		detectLatestRelease: func(repo string) (semver.Version, bool, error) {
			return semver.MustParse("0.1.78"), true, nil
		},
		now:            func() time.Time { return currentTime },
		selfUpdateRepo: "soultech67/opctl",
		warningWriter:  &warningWriter,
	})

	if warningWriter.String() != "opctl 0.1.78 is available. Run 'opctl self-update' to update.\n" {
		t.Fatalf("expected update hint to be written to warning writer, got %q", warningWriter.String())
	}
}

func TestGetUpdateHintUsesFreshCachedNewerRelease(t *testing.T) {
	dataDir := t.TempDir()
	currentTime := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	writeUpdateHintCache(dataDir, updateHintCache{
		LastCheckedAt: currentTime.Add(-1 * time.Hour),
		LatestVersion: "0.1.78",
	})

	detectorCalled := false
	hint := getUpdateHint(updateHintConfig{
		args:           []string{"run"},
		command:        runnableCommand("run"),
		currentVersion: "0.1.77",
		dataDir:        dataDir,
		detectLatestRelease: func(repo string) (semver.Version, bool, error) {
			detectorCalled = true
			return semver.Version{}, false, nil
		},
		now:            func() time.Time { return currentTime },
		selfUpdateRepo: "soultech67/opctl",
	})

	if detectorCalled {
		t.Fatal("expected fresh cache to avoid release detector")
	}
	if hint != "opctl 0.1.78 is available. Run 'opctl self-update' to update." {
		t.Fatalf("expected cached update hint, got %q", hint)
	}
}

func TestGetUpdateHintSkipsFreshCacheWithoutNewerRelease(t *testing.T) {
	dataDir := t.TempDir()
	currentTime := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	writeUpdateHintCache(dataDir, updateHintCache{
		LastCheckedAt: currentTime.Add(-1 * time.Hour),
		LatestVersion: "0.1.77",
	})

	detectorCalled := false
	hint := getUpdateHint(updateHintConfig{
		args:           []string{"run"},
		command:        runnableCommand("run"),
		currentVersion: "0.1.77",
		dataDir:        dataDir,
		detectLatestRelease: func(repo string) (semver.Version, bool, error) {
			detectorCalled = true
			return semver.MustParse("0.1.78"), true, nil
		},
		now:            func() time.Time { return currentTime },
		selfUpdateRepo: "soultech67/opctl",
	})

	if detectorCalled {
		t.Fatal("expected fresh cache to avoid release detector")
	}
	if hint != "" {
		t.Fatalf("expected no update hint, got %q", hint)
	}
}

func TestGetUpdateHintRefreshesStaleCache(t *testing.T) {
	dataDir := t.TempDir()
	currentTime := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	writeUpdateHintCache(dataDir, updateHintCache{
		LastCheckedAt: currentTime.Add(-25 * time.Hour),
		LatestVersion: "0.1.77",
	})

	hint := getUpdateHint(updateHintConfig{
		args:           []string{"run"},
		command:        runnableCommand("run"),
		currentVersion: "0.1.77",
		dataDir:        dataDir,
		detectLatestRelease: func(repo string) (semver.Version, bool, error) {
			return semver.MustParse("0.1.78"), true, nil
		},
		now:            func() time.Time { return currentTime },
		selfUpdateRepo: "soultech67/opctl",
	})

	if hint != "opctl 0.1.78 is available. Run 'opctl self-update' to update." {
		t.Fatalf("expected refreshed update hint, got %q", hint)
	}
}

func TestGetUpdateHintFallsBackToStaleCachedNewerReleaseOnDetectorError(t *testing.T) {
	dataDir := t.TempDir()
	currentTime := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	writeUpdateHintCache(dataDir, updateHintCache{
		LastCheckedAt: currentTime.Add(-25 * time.Hour),
		LatestVersion: "0.1.78",
	})

	hint := getUpdateHint(updateHintConfig{
		args:           []string{"run"},
		command:        runnableCommand("run"),
		currentVersion: "0.1.77",
		dataDir:        dataDir,
		detectLatestRelease: func(repo string) (semver.Version, bool, error) {
			return semver.Version{}, false, errors.New("github unavailable")
		},
		now:            func() time.Time { return currentTime },
		selfUpdateRepo: "soultech67/opctl",
	})

	if hint != "opctl 0.1.78 is available. Run 'opctl self-update' to update." {
		t.Fatalf("expected stale cached update hint, got %q", hint)
	}

	cache := readUpdateHintCache(dataDir)
	if !cache.LastCheckedAt.Equal(currentTime) {
		t.Fatalf("expected detector error to refresh last checked time, got %s", cache.LastCheckedAt)
	}
}

func TestGetUpdateHintSkipsUnsupportedCommandsAndVersions(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		command        *cobra.Command
		currentVersion string
		selfUpdateRepo string
	}{
		{
			name:           "missing version",
			args:           []string{"run"},
			command:        runnableCommand("run"),
			currentVersion: "",
			selfUpdateRepo: "soultech67/opctl",
		},
		{
			name:           "development version",
			args:           []string{"run"},
			command:        runnableCommand("run"),
			currentVersion: "0.0.0",
			selfUpdateRepo: "soultech67/opctl",
		},
		{
			name:           "missing repo",
			args:           []string{"run"},
			command:        runnableCommand("run"),
			currentVersion: "0.1.77",
			selfUpdateRepo: "",
		},
		{
			name:           "self update command",
			args:           []string{"self-update"},
			command:        runnableCommand("self-update"),
			currentVersion: "0.1.77",
			selfUpdateRepo: "soultech67/opctl",
		},
		{
			name:           "help flag",
			args:           []string{"run", "--help"},
			command:        runnableCommand("run"),
			currentVersion: "0.1.77",
			selfUpdateRepo: "soultech67/opctl",
		},
		{
			name:           "version flag",
			args:           []string{"-v"},
			command:        runnableCommand("opctl"),
			currentVersion: "0.1.77",
			selfUpdateRepo: "soultech67/opctl",
		},
		{
			name:           "non runnable command",
			args:           []string{"node"},
			command:        &cobra.Command{Use: "node"},
			currentVersion: "0.1.77",
			selfUpdateRepo: "soultech67/opctl",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectorCalled := false
			hint := getUpdateHint(updateHintConfig{
				args:           test.args,
				command:        test.command,
				currentVersion: test.currentVersion,
				dataDir:        t.TempDir(),
				detectLatestRelease: func(repo string) (semver.Version, bool, error) {
					detectorCalled = true
					return semver.MustParse("0.1.78"), true, nil
				},
				now:            func() time.Time { return time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC) },
				selfUpdateRepo: test.selfUpdateRepo,
			})

			if detectorCalled {
				t.Fatal("expected update detector not to be called")
			}
			if hint != "" {
				t.Fatalf("expected no update hint, got %q", hint)
			}
		})
	}
}

func runnableCommand(name string) *cobra.Command {
	return &cobra.Command{
		Use: name,
		Run: func(cmd *cobra.Command, args []string) {},
	}
}

// TestUpdateHintCommandTreeCoverage walks the real CLI command tree and
// asserts, for every runnable command, whether it gets the update hint:
// streaming/long-blocking commands must not (the hint would interleave with
// live output as noise), everything else must (as the final line of output).
func TestUpdateHintCommandTreeCoverage(t *testing.T) {
	rootCmd, err := NewRootCmd()
	if err != nil {
		t.Fatalf("NewRootCmd() error: %v", err)
	}

	// expected skips, stated independently of the production skip set
	skipped := map[string]bool{
		"opctl run":              true, // streams op output; hint seen mid-log-stream
		"opctl events":           true, // streams forever
		"opctl node create":      true, // foreground daemon; blocks
		"opctl doctor tail-logs": true, // follows the log; blocks
		"opctl self-update":      true, // replaces the binary; hint is redundant
	}

	seen := map[string]bool{}
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Runnable() && !cmd.Hidden {
			path := cmd.CommandPath()
			seen[path] = true

			got := shouldCheckForUpdateHint(cmd, nil)
			want := !skipped[path]
			if got != want {
				t.Errorf("%s: shouldCheckForUpdateHint = %v, want %v", path, got, want)
			}
		}
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)

	// commands this test specifically promises coverage for
	for _, path := range []string{
		"opctl auth add",
		"opctl auth ls",
		"opctl auth remove",
		"opctl container delete",
		"opctl container down",
		"opctl container ls",
		"opctl container prune",
		"opctl doctor log-level",
		"opctl doctor logs",
		"opctl doctor tail-logs",
		"opctl events",
		"opctl ls",
		"opctl node container ls",
		"opctl node create",
		"opctl node delete",
		"opctl node kill",
		"opctl op create",
		"opctl op install",
		"opctl op kill",
		"opctl op validate",
		"opctl run",
		"opctl self-update",
		"opctl ui",
	} {
		if !seen[path] {
			t.Errorf("expected runnable command %q in the CLI tree; command coverage is stale", path)
		}
	}
}

// TestUpdateHintIsEmittedOnceAfterCommandOutput mirrors Execute()'s wiring:
// the hint is printed only after the executed command has returned, so it is
// the final line of output — and it appears exactly once.
func TestUpdateHintIsEmittedOnceAfterCommandOutput(t *testing.T) {
	out := &bytes.Buffer{}

	rootCmd := &cobra.Command{Use: "opctl"}
	rootCmd.AddCommand(&cobra.Command{
		Use: "ls",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(out, "command output")
		},
	})
	rootCmd.SetArgs([]string{"ls"})

	executedCmd, err := rootCmd.ExecuteContextC(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	maybePrintUpdateHint(updateHintConfig{
		args:           []string{"ls"},
		command:        executedCmd,
		currentVersion: "0.1.77",
		dataDir:        t.TempDir(),
		detectLatestRelease: func(repo string) (semver.Version, bool, error) {
			return semver.MustParse("0.1.78"), true, nil
		},
		now:            func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) },
		selfUpdateRepo: "opctl/opctl",
		warningWriter:  out,
	})

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected exactly 2 output lines (command output, then hint), got %d: %q", len(lines), out.String())
	}
	if lines[0] != "command output" {
		t.Errorf("expected command output first, got %q", lines[0])
	}
	if want := formatUpdateHint("0.1.78"); lines[1] != want {
		t.Errorf("expected update hint %q as the final line, got %q", want, lines[1])
	}
	if got := strings.Count(out.String(), "is available"); got != 1 {
		t.Errorf("expected the update hint exactly once, found %d occurrences", got)
	}
}
