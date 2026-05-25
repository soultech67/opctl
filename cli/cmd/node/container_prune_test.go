package node

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/opctl/opctl/sdks/go/node/containerruntime"
)

func TestSelectContainersToPruneExcludesRunningAndUnknownState(t *testing.T) {
	containers := []containerruntime.Container{
		{ID: "running-1", State: "running"},
		{ID: "exited-1", State: "exited"},
		{ID: "created-1", State: "created"},
		{ID: "dead-1", State: "dead"},
		{ID: "paused-1", State: "paused"},
		{ID: "restarting-1", State: "restarting"},
		{ID: "no-state", State: ""},
	}

	prunable := selectContainersToPrune(containers)

	expectedIDs := []string{"exited-1", "created-1", "dead-1", "paused-1", "restarting-1"}
	if len(prunable) != len(expectedIDs) {
		t.Fatalf("expected %d prunable containers, got %d: %+v", len(expectedIDs), len(prunable), prunable)
	}
	for i, id := range expectedIDs {
		if prunable[i].ID != id {
			t.Fatalf("expected prunable[%d].ID == %q, got %q", i, id, prunable[i].ID)
		}
	}
}

func TestPruneContainersNoMatchesReportsAndExits(t *testing.T) {
	stdout := bytes.Buffer{}
	deleteCalls := 0

	err := pruneContainers(
		context.Background(),
		&stdout,
		[]containerruntime.Container{
			{ID: "running-1", State: "running"},
		},
		false,
		func(prompt string) (string, error) {
			t.Fatalf("confirm should not be called when nothing is prunable")
			return "", nil
		},
		func(ctx context.Context, target string) error {
			deleteCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if deleteCalls != 0 {
		t.Fatalf("expected zero delete calls, got %d", deleteCalls)
	}
	if !strings.Contains(stdout.String(), "no stopped opctl-managed containers to prune") {
		t.Fatalf("expected empty-set message, got %q", stdout.String())
	}
}

func TestPruneContainersPromptsAndDeletesWhenConfirmed(t *testing.T) {
	stdout := bytes.Buffer{}
	deleted := []string{}

	containers := []containerruntime.Container{
		{ID: "running-1", Name: "opctl_keep_aaa", State: "running"},
		{ID: "exited-1", Name: "opctl_drop_bbb", State: "exited", Status: "Exited (0) 2 hours ago"},
		{ID: "created-1", Name: "opctl_drop_ccc", State: "created", Status: "Created"},
	}

	err := pruneContainers(
		context.Background(),
		&stdout,
		containers,
		false,
		func(prompt string) (string, error) {
			if !strings.Contains(prompt, "Are you sure") {
				t.Fatalf("expected confirmation prompt, got %q", prompt)
			}
			return "y", nil
		},
		func(ctx context.Context, target string) error {
			deleted = append(deleted, target)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedDeleted := []string{"exited-1", "created-1"}
	if len(deleted) != len(expectedDeleted) {
		t.Fatalf("expected %d deletes, got %d: %v", len(expectedDeleted), len(deleted), deleted)
	}
	for i, target := range expectedDeleted {
		if deleted[i] != target {
			t.Fatalf("expected deleted[%d] == %q, got %q", i, target, deleted[i])
		}
	}

	output := stdout.String()
	for _, expectedOutput := range []string{
		"WARNING! This will remove 2 stopped opctl-managed container(s):",
		"drop-bbb",
		"drop-ccc",
		"Exited (0) 2 hours ago",
		"removed 2 stopped opctl-managed container(s)",
	} {
		if !strings.Contains(output, expectedOutput) {
			t.Fatalf("expected output to include %q, got %q", expectedOutput, output)
		}
	}
	if strings.Contains(output, "keep-aaa") {
		t.Fatalf("expected running container not to be listed: got %q", output)
	}
}

func TestPruneContainersCancelsOnNegativeAnswer(t *testing.T) {
	stdout := bytes.Buffer{}
	deleteCalls := 0

	err := pruneContainers(
		context.Background(),
		&stdout,
		[]containerruntime.Container{
			{ID: "exited-1", State: "exited"},
		},
		false,
		func(prompt string) (string, error) {
			return "n", nil
		},
		func(ctx context.Context, target string) error {
			deleteCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if deleteCalls != 0 {
		t.Fatalf("expected zero delete calls after cancellation, got %d", deleteCalls)
	}
	if !strings.Contains(stdout.String(), "prune cancelled") {
		t.Fatalf("expected cancellation message, got %q", stdout.String())
	}
}

func TestPruneContainersForceSkipsConfirmation(t *testing.T) {
	stdout := bytes.Buffer{}
	deleted := []string{}

	err := pruneContainers(
		context.Background(),
		&stdout,
		[]containerruntime.Container{
			{ID: "exited-1", Name: "opctl_x_bbb", State: "exited"},
		},
		true,
		func(prompt string) (string, error) {
			t.Fatalf("confirm should not be called when force=true")
			return "", nil
		},
		func(ctx context.Context, target string) error {
			deleted = append(deleted, target)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "exited-1" {
		t.Fatalf("expected one delete of exited-1, got %v", deleted)
	}
}

func TestPruneContainersReportsDeleteFailures(t *testing.T) {
	stdout := bytes.Buffer{}

	err := pruneContainers(
		context.Background(),
		&stdout,
		[]containerruntime.Container{
			{ID: "exited-1", Name: "opctl_drop_bbb", State: "exited"},
		},
		true,
		func(prompt string) (string, error) { return "y", nil },
		func(ctx context.Context, target string) error {
			return errors.New("boom")
		},
	)
	if err == nil {
		t.Fatalf("expected an error")
	}
	if !strings.Contains(err.Error(), "drop-bbb") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to mention container and underlying cause, got %v", err)
	}
}

func TestIsAffirmativeAcceptsYesVariants(t *testing.T) {
	for _, in := range []string{"y", "Y", "yes", "YES", "  yes  "} {
		if !isAffirmative(in) {
			t.Fatalf("expected %q to be affirmative", in)
		}
	}
	for _, in := range []string{"", "n", "no", "maybe", "yep"} {
		if isAffirmative(in) {
			t.Fatalf("expected %q NOT to be affirmative", in)
		}
	}
}
