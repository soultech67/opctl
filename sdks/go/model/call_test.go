package model

import (
	"encoding/json"
	"testing"
)

// Events are persisted to the event store as JSON and replayed on
// subscription, while live subscribers receive the in-memory struct. Every
// field must survive that round-trip unchanged, or the same event has a
// different shape depending on which delivery path a subscriber hits (a race).
func TestContainerCallJSONRoundTripPreservesEmptyMaps(t *testing.T) {
	original := ContainerCall{
		Cmd:     []string{},
		Dirs:    map[string]string{},
		Files:   map[string]string{},
		Sockets: map[string]string{},
		Volumes: map[string]string{},
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded ContainerCall
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	for name, m := range map[string]map[string]string{
		"Dirs":    decoded.Dirs,
		"Files":   decoded.Files,
		"Sockets": decoded.Sockets,
		"Volumes": decoded.Volumes,
	} {
		if m == nil {
			t.Errorf("%s: empty map became nil after JSON round-trip (marshaled: %s)", name, encoded)
		}
	}
}
