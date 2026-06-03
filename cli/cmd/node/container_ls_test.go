package node

import (
	"testing"

	"github.com/opctl/opctl/sdks/go/node/containerruntime"
)

func TestFilterContainersByName(t *testing.T) {
	containers := []containerruntime.Container{
		{ID: "a", Name: "opctl_artifacts-api_8a0b5a48"},
		{ID: "b", Name: "opctl_artifacts-db-bootstrap_bbb"},
		{ID: "c", Name: "opctl_lium-web_ccc"},
	}

	cases := []struct {
		filter      string
		expectedIDs []string
	}{
		{"", []string{"a", "b", "c"}},          // empty -> all
		{"artifacts-api", []string{"a"}},       // bare name, opctl_ prefix implied
		{"artifacts", []string{"a", "b"}},      // substring matches multiple
		{"ARTIFACTS-API", []string{"a"}},       // case-insensitive
		{"artifacts_api", []string{"a"}},       // `_` and `-` interchangeable
		{"lium-web", []string{"c"}},            //
		{"opctl_artifacts-api", []string{"a"}}, // typing the prefix still works (raw-name match)
		{"nope", nil},                          // no match
		{"  artifacts-api  ", []string{"a"}},   // trimmed
	}

	for _, tc := range cases {
		got := filterContainersByName(containers, tc.filter)
		gotIDs := []string{}
		for _, c := range got {
			gotIDs = append(gotIDs, c.ID)
		}
		if len(gotIDs) != len(tc.expectedIDs) {
			t.Fatalf("filter %q: expected %v, got %v", tc.filter, tc.expectedIDs, gotIDs)
		}
		for i, id := range tc.expectedIDs {
			if gotIDs[i] != id {
				t.Fatalf("filter %q: expected %v, got %v", tc.filter, tc.expectedIDs, gotIDs)
			}
		}
	}
}
