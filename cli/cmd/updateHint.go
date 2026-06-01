package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/opctl/opctl/cli/internal/clicolorer"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
)

const (
	updateHintCacheFile     = "update-hint.json"
	updateHintCheckInterval = 24 * time.Hour
)

type latestReleaseDetector func(repo string) (semver.Version, bool, error)

type updateHintConfig struct {
	args                []string
	cliColorer          clicolorer.CliColorer
	command             *cobra.Command
	currentVersion      string
	dataDir             string
	detectLatestRelease latestReleaseDetector
	now                 func() time.Time
	selfUpdateRepo      string
	warningWriter       io.Writer
}

type updateHintCache struct {
	LastCheckedAt time.Time `json:"lastCheckedAt"`
	LatestVersion string    `json:"latestVersion,omitempty"`
}

func maybePrintUpdateHint(config updateHintConfig) {
	hint := getUpdateHint(config)
	if hint == "" {
		return
	}

	if config.cliColorer != nil {
		hint = config.cliColorer.Attention(hint)
	}

	var warningWriter io.Writer = os.Stderr
	if config.warningWriter != nil {
		warningWriter = config.warningWriter
	}

	fmt.Fprintln(warningWriter, hint)
}

func getUpdateHint(config updateHintConfig) string {
	if !shouldCheckForUpdateHint(config.command, config.args) {
		return ""
	}

	currentVersion, ok := parseUpdateHintVersion(config.currentVersion)
	if !ok || currentVersion.Equals(semver.Version{}) {
		return ""
	}

	if strings.TrimSpace(config.selfUpdateRepo) == "" {
		return ""
	}

	now := time.Now
	if config.now != nil {
		now = config.now
	}

	detectLatestRelease := detectLatestSelfUpdateRelease
	if config.detectLatestRelease != nil {
		detectLatestRelease = config.detectLatestRelease
	}

	currentTime := now()
	cache := readUpdateHintCache(config.dataDir)
	cachedHint := updateHintForVersion(cache.LatestVersion, currentVersion)
	if isUpdateHintCacheFresh(cache, currentTime) {
		return cachedHint
	}

	latestVersion, found, err := detectLatestRelease(config.selfUpdateRepo)
	cache.LastCheckedAt = currentTime
	if err != nil {
		writeUpdateHintCache(config.dataDir, cache)
		return cachedHint
	}

	if found {
		cache.LatestVersion = latestVersion.String()
	} else {
		cache.LatestVersion = ""
	}

	writeUpdateHintCache(config.dataDir, cache)

	if found && latestVersion.GT(currentVersion) {
		return formatUpdateHint(latestVersion.String())
	}

	return ""
}

func shouldCheckForUpdateHint(command *cobra.Command, args []string) bool {
	if command == nil || !command.Runnable() || command.Name() == "self-update" {
		return false
	}

	for _, arg := range args {
		switch arg {
		case "help", "-h", "--help", "-v", "--version":
			return false
		}
	}

	return true
}

func parseUpdateHintVersion(version string) (semver.Version, bool) {
	parsedVersion, err := semver.Make(strings.TrimPrefix(strings.TrimSpace(version), "v"))
	if err != nil {
		return semver.Version{}, false
	}

	return parsedVersion, true
}

func detectLatestSelfUpdateRelease(repo string) (semver.Version, bool, error) {
	release, found, err := selfupdate.DetectLatest(repo)
	if err != nil || !found {
		return semver.Version{}, found, err
	}

	return release.Version, true, nil
}

func updateHintForVersion(latestVersion string, currentVersion semver.Version) string {
	parsedLatestVersion, ok := parseUpdateHintVersion(latestVersion)
	if !ok || !parsedLatestVersion.GT(currentVersion) {
		return ""
	}

	return formatUpdateHint(parsedLatestVersion.String())
}

func formatUpdateHint(latestVersion string) string {
	return fmt.Sprintf("opctl %s is available. Run 'opctl self-update' to update.", latestVersion)
}

func isUpdateHintCacheFresh(cache updateHintCache, currentTime time.Time) bool {
	return !cache.LastCheckedAt.IsZero() && currentTime.Sub(cache.LastCheckedAt) < updateHintCheckInterval
}

func readUpdateHintCache(dataDir string) updateHintCache {
	cacheFilePath := getUpdateHintCachePath(dataDir)
	if cacheFilePath == "" {
		return updateHintCache{}
	}

	cacheBytes, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return updateHintCache{}
	}

	cache := updateHintCache{}
	if err := json.Unmarshal(cacheBytes, &cache); err != nil {
		return updateHintCache{}
	}

	return cache
}

func writeUpdateHintCache(dataDir string, cache updateHintCache) {
	cacheFilePath := getUpdateHintCachePath(dataDir)
	if cacheFilePath == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(cacheFilePath), 0700); err != nil {
		return
	}

	cacheBytes, err := json.Marshal(cache)
	if err != nil {
		return
	}

	_ = os.WriteFile(cacheFilePath, cacheBytes, 0600)
}

func getUpdateHintCachePath(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		return ""
	}

	return filepath.Join(dataDir, updateHintCacheFile)
}
