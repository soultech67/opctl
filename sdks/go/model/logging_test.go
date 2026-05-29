package model

import "testing"

func TestNormalizeLogLevel(t *testing.T) {
	cases := []struct {
		in    string
		want  string
		valid bool
	}{
		{"debug", LogLevelDebug, true},
		{"INFO", LogLevelInfo, true},
		{" Warn ", LogLevelWarn, true},
		{"warning", LogLevelWarn, true},
		{"WARNING", LogLevelWarn, true},
		{"error", LogLevelError, true},
		{"", "", false},
		{"trace", "trace", false},
		{"verbose", "verbose", false},
	}

	for _, c := range cases {
		got, ok := NormalizeLogLevel(c.in)
		if ok != c.valid {
			t.Errorf("NormalizeLogLevel(%q) valid = %v, want %v", c.in, ok, c.valid)
		}
		if c.valid && got != c.want {
			t.Errorf("NormalizeLogLevel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
