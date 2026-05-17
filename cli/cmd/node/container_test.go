package node

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/opctl/opctl/sdks/go/node/containerruntime"
)

func TestSelectContainersToDeleteReturnsOnlySelectedContainers(t *testing.T) {
	stdout := bytes.Buffer{}
	containers := []containerruntime.Container{
		{
			ID:   "first-container-id",
			Name: "opctl_astro-local-localstack_a345dfdf7",
		},
		{
			ID:   "second-container-id",
			Name: "opctl_astro-local-localstack_b323fd345",
		},
	}

	selectedContainers, err := selectContainersToDelete(
		&stdout,
		containers,
		1,
		true,
		func(prompt string) (string, error) {
			if !strings.Contains(prompt, "[1-2]") {
				t.Fatalf("expected prompt to include selection range, got %q", prompt)
			}
			return "2, 1, 2", nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(selectedContainers) != 2 {
		t.Fatalf("expected two selected containers, got %d", len(selectedContainers))
	}
	if selectedContainers[0].ID != "second-container-id" {
		t.Fatalf("expected first selected container to be second-container-id, got %q", selectedContainers[0].ID)
	}
	if selectedContainers[1].ID != "first-container-id" {
		t.Fatalf("expected second selected container to be first-container-id, got %q", selectedContainers[1].ID)
	}

	output := stdout.String()
	for _, expectedOutput := range []string{
		"multiple containers match label",
		"[1] [ ] astro-local-localstack-a345dfdf7 started unknown",
		"[2] [ ] astro-local-localstack-b323fd345 started unknown",
	} {
		if !strings.Contains(output, expectedOutput) {
			t.Fatalf("expected output to include %q, got %q", expectedOutput, output)
		}
	}
}

func TestSelectContainersToDeleteRejectsMultipleMatchesWithoutInteractiveTerminal(t *testing.T) {
	_, err := selectContainersToDelete(
		&bytes.Buffer{},
		[]containerruntime.Container{
			{ID: "first-container-id"},
			{ID: "second-container-id"},
		},
		1,
		false,
		func(prompt string) (string, error) {
			t.Fatalf("prompt should not be called")
			return "", nil
		},
	)

	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "multiple containers match labels") {
		t.Fatalf("expected multiple match error, got %v", err)
	}
}

func TestParseContainerSelection(t *testing.T) {
	selectedIndexes, err := parseContainerSelection(" 2, 1,2 ", 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(selectedIndexes) != 2 {
		t.Fatalf("expected two selected indexes, got %d", len(selectedIndexes))
	}
	if selectedIndexes[0] != 1 || selectedIndexes[1] != 0 {
		t.Fatalf("expected selected indexes [1 0], got %v", selectedIndexes)
	}
}

func TestParseContainerSelectionRejectsInvalidValues(t *testing.T) {
	for _, rawSelection := range []string{"", "0", "3", "abc", "1,"} {
		if _, err := parseContainerSelection(rawSelection, 2); err == nil {
			t.Fatalf("expected %q to return an error", rawSelection)
		}
	}
}

func TestWriteContainerListIncludesRelevantDeleteLabels(t *testing.T) {
	stdout := bytes.Buffer{}
	startedAt := time.Date(2026, 5, 17, 17, 1, 32, 0, time.UTC)

	err := writeContainerList(
		&stdout,
		[]containerruntime.Container{
			{
				ID:        "2a647646e9cc4ef4940f52ad944f5657",
				Name:      "opctl_astro-local-localstack_a345dfdf7",
				Image:     "localstack/localstack-pro:latest",
				StartedAt: startedAt,
				Labels: map[string]string{
					"opctl.container-id":   "2a647646e9cc4ef4940f52ad944f5657",
					"opctl.container-name": "astro-local-localstack",
					"opctl.image-ref":      "localstack/localstack-pro:latest",
					"opctl.managed":        "true",
					"external.label":       "not-for-delete",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := stdout.String()
	for _, expectedOutput := range []string{
		"NAME",
		"astro-local-localstack-a345dfdf7",
		"2a647646e9cc",
		"localstack/localstack-pro:latest",
		"\nDELETE LABELS\n",
		"\n  container-id=2a647646e9cc4ef4940f52ad944f5657",
		"\n  container-name=astro-local-localstack",
		"\n  image-ref=localstack/localstack-pro:latest",
	} {
		if !strings.Contains(output, expectedOutput) {
			t.Fatalf("expected output to include %q, got %q", expectedOutput, output)
		}
	}
	for _, unexpectedOutput := range []string{
		"opctl.container-id=2a647646e9cc4ef4940f52ad944f5657",
		"opctl.container-name=astro-local-localstack",
		"opctl.image-ref=localstack/localstack-pro:latest",
		"opctl.managed=true",
		"external.label=not-for-delete",
	} {
		if strings.Contains(output, unexpectedOutput) {
			t.Fatalf("expected output not to include %q, got %q", unexpectedOutput, output)
		}
	}

	headerLine := strings.Split(output, "\n")[0]
	if strings.Contains(headerLine, "DELETE LABELS") {
		t.Fatalf("expected DELETE LABELS not to be in the table header, got %q", headerLine)
	}
}

func TestFormatContainerDeleteLabelsReturnsEmptySliceWhenNoUsefulLabelsExist(t *testing.T) {
	actualLabels := formatContainerDeleteLabels(map[string]string{
		"opctl.managed":  "true",
		"external.label": "not-for-delete",
	})

	if len(actualLabels) != 0 {
		t.Fatalf("expected no labels, got %q", actualLabels)
	}
}

func TestNormalizeContainerDeleteLabelFiltersAcceptsListShorthand(t *testing.T) {
	actualLabels := normalizeContainerDeleteLabelFilters([]string{
		"container-id=2a647646e9cc4ef4940f52ad944f5657",
		"container-name=astro-local-localstack",
		"image-ref=docker.io/localstack/localstack-pro:latest",
		"opctl.container-name=already-full",
		"com.example.owner=scott",
	})

	expectedLabels := []string{
		"opctl.container-id=2a647646e9cc4ef4940f52ad944f5657",
		"opctl.container-name=astro-local-localstack",
		"opctl.image-ref=docker.io/localstack/localstack-pro:latest",
		"opctl.container-name=already-full",
		"com.example.owner=scott",
	}
	if len(actualLabels) != len(expectedLabels) {
		t.Fatalf("expected %d labels, got %d: %q", len(expectedLabels), len(actualLabels), actualLabels)
	}
	for i, expectedLabel := range expectedLabels {
		if actualLabels[i] != expectedLabel {
			t.Fatalf("expected label %d to be %q, got %q", i, expectedLabel, actualLabels[i])
		}
	}
}
