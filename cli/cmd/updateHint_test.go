package cmd

import (
	"bytes"
	"errors"
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
