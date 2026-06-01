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

// sampleContainerWithLabels is a single representative container used by the
// writeContainerList variant tests below.
func sampleContainerWithLabels() containerruntime.Container {
	return containerruntime.Container{
		ID:        "2a647646e9cc4ef4940f52ad944f5657",
		Name:      "opctl_astro-local-localstack_a345dfdf7",
		Image:     "localstack/localstack-pro:latest",
		State:     "running",
		Status:    "Up 5 minutes",
		StartedAt: time.Date(2026, 5, 17, 17, 1, 32, 0, time.UTC),
		Labels: map[string]string{
			"opctl.container-id":   "2a647646e9cc4ef4940f52ad944f5657",
			"opctl.container-name": "astro-local-localstack",
			"opctl.image-ref":      "localstack/localstack-pro:latest",
			"opctl.managed":        "true",
			"external.label":       "not-for-delete",
		},
	}
}

func TestWriteContainerListDefaultOmitsImageAndLabels(t *testing.T) {
	stdout := bytes.Buffer{}

	err := writeContainerList(
		&stdout,
		[]containerruntime.Container{sampleContainerWithLabels()},
		containerListOptions{},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := stdout.String()
	for _, expectedOutput := range []string{
		"NAME",
		"STATUS",
		"Up 5 minutes",
		"astro-local-localstack-a345dfdf7",
		"2a647646e9cc",
	} {
		if !strings.Contains(output, expectedOutput) {
			t.Fatalf("expected output to include %q, got %q", expectedOutput, output)
		}
	}
	for _, unexpectedOutput := range []string{
		"IMAGE",
		"localstack/localstack-pro:latest",
		"DELETE LABELS",
		"container-id=2a647646e9cc4ef4940f52ad944f5657",
		"container-name=astro-local-localstack",
		"image-ref=localstack/localstack-pro:latest",
	} {
		if strings.Contains(output, unexpectedOutput) {
			t.Fatalf("expected output NOT to include %q, got %q", unexpectedOutput, output)
		}
	}

	headerLine := strings.Split(output, "\n")[0]
	if strings.Contains(headerLine, "IMAGE") {
		t.Fatalf("expected IMAGE not to be in the default table header, got %q", headerLine)
	}
}

func TestWriteContainerListWithImagesIncludesImageColumn(t *testing.T) {
	stdout := bytes.Buffer{}

	err := writeContainerList(
		&stdout,
		[]containerruntime.Container{sampleContainerWithLabels()},
		containerListOptions{IncludeImage: true},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := stdout.String()
	headerLine := strings.Split(output, "\n")[0]
	if !strings.Contains(headerLine, "IMAGE") {
		t.Fatalf("expected IMAGE in table header, got %q", headerLine)
	}

	// IMAGE column should sit between ID and STATUS in the header.
	idIdx := strings.Index(headerLine, "ID")
	imageIdx := strings.Index(headerLine, "IMAGE")
	statusIdx := strings.Index(headerLine, "STATUS")
	if !(idIdx >= 0 && imageIdx > idIdx && statusIdx > imageIdx) {
		t.Fatalf("expected column order NAME, ID, IMAGE, STATUS, STARTED, got header %q", headerLine)
	}

	if !strings.Contains(output, "localstack/localstack-pro:latest") {
		t.Fatalf("expected image value in output, got %q", output)
	}
	if strings.Contains(output, "DELETE LABELS") {
		t.Fatalf("expected DELETE LABELS NOT to appear without -v, got %q", output)
	}
}

func TestWriteContainerListVerboseAppendsDeleteLabelsSection(t *testing.T) {
	stdout := bytes.Buffer{}

	err := writeContainerList(
		&stdout,
		[]containerruntime.Container{sampleContainerWithLabels()},
		containerListOptions{Verbose: true},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := stdout.String()
	for _, expectedOutput := range []string{
		"\nDELETE LABELS\n",
		"  astro-local-localstack-a345dfdf7\n",
		"    container-id=2a647646e9cc4ef4940f52ad944f5657",
		"    container-name=astro-local-localstack",
		"    image-ref=localstack/localstack-pro:latest",
	} {
		if !strings.Contains(output, expectedOutput) {
			t.Fatalf("expected output to include %q, got %q", expectedOutput, output)
		}
	}
	for _, unexpectedOutput := range []string{
		"opctl.container-id=2a647646e9cc4ef4940f52ad944f5657",
		"opctl.managed=true",
		"external.label=not-for-delete",
	} {
		if strings.Contains(output, unexpectedOutput) {
			t.Fatalf("expected output NOT to include %q, got %q", unexpectedOutput, output)
		}
	}

	// The labels section must come AFTER the table (i.e. after the row line).
	rowIdx := strings.Index(output, "astro-local-localstack-a345dfdf7  ")
	labelsIdx := strings.Index(output, "DELETE LABELS")
	if rowIdx < 0 || labelsIdx < 0 || labelsIdx < rowIdx {
		t.Fatalf("expected DELETE LABELS to appear after the table row; got %q", output)
	}
}

func TestWriteContainerListVerboseSkipsLabelsSectionWhenNoneAvailable(t *testing.T) {
	stdout := bytes.Buffer{}

	err := writeContainerList(
		&stdout,
		[]containerruntime.Container{
			{ID: "abc", Status: "Up 5 minutes"},
		},
		containerListOptions{Verbose: true},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if strings.Contains(stdout.String(), "DELETE LABELS") {
		t.Fatalf("expected DELETE LABELS section to be skipped when no labels are present, got %q", stdout.String())
	}
}

func TestWriteContainerListUsesSpacePadCharForAlignment(t *testing.T) {
	stdout := bytes.Buffer{}

	err := writeContainerList(
		&stdout,
		[]containerruntime.Container{
			{
				Name:   "opctl_short_a1",
				ID:     "1111111111111111",
				Status: "Up 5 minutes",
			},
			{
				Name:   "opctl_much-longer-name-here_b2",
				ID:     "2222222222222222",
				Status: "Exited (0) 3 minutes ago",
			},
		},
		containerListOptions{},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// With padchar=' ', no rendered row should contain a literal tab. With
	// padchar='\t' the rendered output is full of them.
	output := stdout.String()
	if strings.ContainsRune(output, '\t') {
		t.Fatalf("expected no literal tab characters in rendered table (padchar should be space), got %q", output)
	}
}

func TestFormatContainerStatusFallsBackThroughStatusStateAndDash(t *testing.T) {
	cases := []struct {
		name     string
		input    containerruntime.Container
		expected string
	}{
		{
			name:     "prefers Status when present",
			input:    containerruntime.Container{Status: "Up 5 minutes", State: "running"},
			expected: "Up 5 minutes",
		},
		{
			name:     "falls back to State when Status is blank",
			input:    containerruntime.Container{State: "exited"},
			expected: "exited",
		},
		{
			name:     "falls back to dash when both are blank",
			input:    containerruntime.Container{},
			expected: "-",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatContainerStatus(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestFilterContainersForListHidesNonRunningByDefault(t *testing.T) {
	containers := []containerruntime.Container{
		{ID: "running-1", State: "running"},
		{ID: "exited-1", State: "exited"},
		{ID: "created-1", State: "created"},
		{ID: "running-2", State: "running"},
		{ID: "no-state", State: ""},
	}

	filtered := filterContainersForList(containers, false)

	expectedIDs := []string{"running-1", "running-2", "no-state"}
	if len(filtered) != len(expectedIDs) {
		t.Fatalf("expected %d filtered containers, got %d: %+v", len(expectedIDs), len(filtered), filtered)
	}
	for i, id := range expectedIDs {
		if filtered[i].ID != id {
			t.Fatalf("expected filtered[%d].ID == %q, got %q", i, id, filtered[i].ID)
		}
	}
}

func TestFilterContainersForListWithAllReturnsEverything(t *testing.T) {
	containers := []containerruntime.Container{
		{ID: "running-1", State: "running"},
		{ID: "exited-1", State: "exited"},
		{ID: "created-1", State: "created"},
	}

	filtered := filterContainersForList(containers, true)
	if len(filtered) != len(containers) {
		t.Fatalf("expected all %d containers, got %d", len(containers), len(filtered))
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
