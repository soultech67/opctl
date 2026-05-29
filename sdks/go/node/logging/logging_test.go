package logging

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opctl/opctl/sdks/go/model"
)

func TestInitConfiguresStateFromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvLevel, "debug")
	t.Setenv(EnvFormat, "json")
	t.Setenv(EnvEnabled, "")
	t.Setenv(EnvFile, "")

	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}

	state := State()
	if !state.Enabled {
		t.Errorf("expected logging enabled by default")
	}
	if state.Level != model.LogLevelDebug {
		t.Errorf("level = %q, want %q", state.Level, model.LogLevelDebug)
	}
	if state.Format != "json" {
		t.Errorf("format = %q, want json", state.Format)
	}
	want := filepath.Join(dir, "logs", "node.log")
	if state.Filepath != want {
		t.Errorf("filepath = %q, want %q", state.Filepath, want)
	}
}

func TestInitEnabledOffAndFileOverride(t *testing.T) {
	dir := t.TempDir()
	override := filepath.Join(dir, "custom", "daemon.log")
	t.Setenv(EnvEnabled, "off")
	t.Setenv(EnvLevel, "")
	t.Setenv(EnvFormat, "")
	t.Setenv(EnvFile, override)

	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}

	state := State()
	if state.Enabled {
		t.Errorf("expected logging disabled when %s=off", EnvEnabled)
	}
	if state.Level != model.LogLevelInfo {
		t.Errorf("default level = %q, want info", state.Level)
	}
	if state.Filepath != override {
		t.Errorf("filepath = %q, want override %q", state.Filepath, override)
	}
}

func TestSetLevel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvLevel, "info")
	t.Setenv(EnvFormat, "")
	t.Setenv(EnvEnabled, "")
	t.Setenv(EnvFile, "")
	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := SetLevel("WARN"); err != nil {
		t.Fatalf("SetLevel(WARN): %v", err)
	}
	if got := State().Level; got != model.LogLevelWarn {
		t.Errorf("level = %q, want warn", got)
	}

	if err := SetLevel("warning"); err != nil {
		t.Fatalf("SetLevel(warning): %v", err)
	}
	if got := State().Level; got != model.LogLevelWarn {
		t.Errorf("warning alias should map to warn, got %q", got)
	}

	err := SetLevel("bogus")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
	var invalid *InvalidLevelError
	if !errors.As(err, &invalid) {
		t.Errorf("expected *InvalidLevelError, got %T", err)
	}
	if got := State().Level; got != model.LogLevelWarn {
		t.Errorf("level changed after invalid set: %q", got)
	}
}

func TestSetEnabledGatesOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvLevel, "debug")
	t.Setenv(EnvFormat, "text")
	t.Setenv(EnvEnabled, "on")
	t.Setenv(EnvFile, "")
	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	logPath := State().Filepath

	slog.Info("first-line-marker")
	afterFirst := fileSize(t, logPath)
	if afterFirst == 0 {
		t.Fatal("expected log file to contain output while enabled")
	}
	if !strings.Contains(readFile(t, logPath), "first-line-marker") {
		t.Error("log file missing expected content")
	}

	SetEnabled(false)
	slog.Info("should-be-dropped")
	if fileSize(t, logPath) != afterFirst {
		t.Error("log file grew while logging disabled")
	}
	if strings.Contains(readFile(t, logPath), "should-be-dropped") {
		t.Error("disabled logging still wrote to file")
	}

	SetEnabled(true)
	slog.Info("third-line-marker")
	if fileSize(t, logPath) <= afterFirst {
		t.Error("log file did not grow after re-enabling")
	}
}

func TestLevelStringRoundTrip(t *testing.T) {
	cases := map[slog.Level]string{
		slog.LevelDebug:     model.LogLevelDebug,
		slog.LevelInfo:      model.LogLevelInfo,
		slog.LevelWarn:      model.LogLevelWarn,
		slog.LevelError:     model.LogLevelError,
		slog.LevelError + 4: model.LogLevelError,
	}
	for level, want := range cases {
		if got := levelString(level); got != want {
			t.Errorf("levelString(%v) = %q, want %q", level, got, want)
		}
	}
}

func TestParseEnabled(t *testing.T) {
	on := []string{"1", "true", "TRUE", "yes", "on"}
	off := []string{"0", "false", "no", "off"}
	for _, v := range on {
		if !parseEnabled(v, false) {
			t.Errorf("parseEnabled(%q) = false, want true", v)
		}
	}
	for _, v := range off {
		if parseEnabled(v, true) {
			t.Errorf("parseEnabled(%q) = true, want false", v)
		}
	}
	if !parseEnabled("garbage", true) {
		t.Error("parseEnabled should return default for unrecognized input")
	}
	if parseEnabled("", false) {
		t.Error("parseEnabled should return default for empty input")
	}
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.Size()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
