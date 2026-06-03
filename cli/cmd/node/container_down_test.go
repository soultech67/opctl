package node

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/opctl/opctl/sdks/go/node/containerruntime"
)

func TestDownContainersSingleRunningShutsDownWithoutPrompt(t *testing.T) {
	stdout := bytes.Buffer{}
	down := []string{}

	err := downContainers(
		context.Background(),
		&stdout,
		"artifacts-api",
		[]containerruntime.Container{
			{ID: "run-1", Name: "opctl_artifacts-api_aaa", State: "running"},
		},
		false, // force
		true,  // interactive
		func(prompt string) (string, error) {
			t.Fatalf("prompt should not be called for a single running match")
			return "", nil
		},
		func(ctx context.Context, target string) error { down = append(down, target); return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(down) != 1 || down[0] != "run-1" {
		t.Fatalf("expected down [run-1], got %v", down)
	}
	if !strings.Contains(stdout.String(), "shut down artifacts-api-aaa") {
		t.Fatalf("expected shutdown confirmation, got %q", stdout.String())
	}
}

func TestDownContainersNoRunningMatchReportsAndHintsPrune(t *testing.T) {
	stdout := bytes.Buffer{}
	downCalls := 0

	err := downContainers(
		context.Background(),
		&stdout,
		"artifacts-api",
		[]containerruntime.Container{
			{ID: "stop-1", Name: "opctl_artifacts-api_aaa", State: "exited"},
		},
		false, true,
		func(prompt string) (string, error) { return "", nil },
		func(ctx context.Context, target string) error { downCalls++; return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if downCalls != 0 {
		t.Fatalf("expected zero down calls, got %d", downCalls)
	}
	out := stdout.String()
	if !strings.Contains(out, "no RUNNING") || !strings.Contains(out, "prune") {
		t.Fatalf("expected a not-running message hinting prune, got %q", out)
	}
}

func TestDownContainersNoMatchAtAllReports(t *testing.T) {
	stdout := bytes.Buffer{}

	err := downContainers(
		context.Background(),
		&stdout,
		"nope",
		[]containerruntime.Container{},
		false, true,
		func(prompt string) (string, error) { return "", nil },
		func(ctx context.Context, target string) error {
			t.Fatalf("should not shut anything down")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `no running opctl-managed container named "nope"`) {
		t.Fatalf("got %q", stdout.String())
	}
}

func TestDownContainersForceShutsDownAllRunningOnly(t *testing.T) {
	stdout := bytes.Buffer{}
	down := []string{}

	err := downContainers(
		context.Background(),
		&stdout,
		"artifacts-api",
		[]containerruntime.Container{
			{ID: "run-1", Name: "opctl_artifacts-api_aaa", State: "running"},
			{ID: "run-2", Name: "opctl_artifacts-api_bbb", State: "running"},
			{ID: "stop-1", Name: "opctl_artifacts-api_ccc", State: "exited"},
		},
		true,  // force
		false, // non-interactive: --force must not need a terminal
		func(prompt string) (string, error) {
			t.Fatalf("prompt should not be called with --force")
			return "", nil
		},
		func(ctx context.Context, target string) error { down = append(down, target); return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(down) != 2 || down[0] != "run-1" || down[1] != "run-2" {
		t.Fatalf("expected down [run-1 run-2] (running only), got %v", down)
	}
}

func TestDownContainersMultipleRunningPromptsAndShutsDownSelected(t *testing.T) {
	stdout := bytes.Buffer{}
	down := []string{}

	err := downContainers(
		context.Background(),
		&stdout,
		"artifacts-api",
		[]containerruntime.Container{
			{ID: "run-1", Name: "opctl_artifacts-api_aaa", State: "running", Status: "Up 5 minutes"},
			{ID: "run-2", Name: "opctl_artifacts-api_bbb", State: "running", Status: "Up 1 hour"},
		},
		false, true,
		func(prompt string) (string, error) {
			if !strings.Contains(prompt, "shut down") {
				t.Fatalf("expected a selection prompt, got %q", prompt)
			}
			return "2", nil
		},
		func(ctx context.Context, target string) error { down = append(down, target); return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(down) != 1 || down[0] != "run-2" {
		t.Fatalf("expected down [run-2], got %v", down)
	}
	if !strings.Contains(stdout.String(), `2 running containers are named "artifacts-api"`) {
		t.Fatalf("expected count header, got %q", stdout.String())
	}
}

func TestDownContainersMultipleRunningNonInteractiveErrors(t *testing.T) {
	stdout := bytes.Buffer{}

	err := downContainers(
		context.Background(),
		&stdout,
		"artifacts-api",
		[]containerruntime.Container{
			{ID: "run-1", State: "running"},
			{ID: "run-2", State: "running"},
		},
		false, false, // no force, non-interactive
		func(prompt string) (string, error) {
			t.Fatalf("should not prompt in a non-interactive terminal")
			return "", nil
		},
		func(ctx context.Context, target string) error {
			t.Fatalf("should not shut anything down")
			return nil
		},
	)
	if err == nil {
		t.Fatalf("expected an error")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected a --force hint, got %v", err)
	}
}

func TestDownContainersReportsFailure(t *testing.T) {
	stdout := bytes.Buffer{}

	err := downContainers(
		context.Background(),
		&stdout,
		"artifacts-api",
		[]containerruntime.Container{
			{ID: "run-1", Name: "opctl_artifacts-api_aaa", State: "running"},
		},
		false, true,
		func(prompt string) (string, error) { return "", nil },
		func(ctx context.Context, target string) error { return errors.New("boom") },
	)
	if err == nil {
		t.Fatalf("expected an error")
	}
	if !strings.Contains(err.Error(), "artifacts-api") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error mentioning the container and the cause, got %v", err)
	}
}
