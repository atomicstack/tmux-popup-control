package menu

import (
	"testing"
	"time"
)

func TestExpandStrftime(t *testing.T) {
	// Use a fixed time for deterministic tests.
	ts := time.Date(2026, 3, 22, 14, 30, 5, 0, time.Local)

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"ISO date", "%F", "2026-03-22"},
		{"year", "%Y", "2026"},
		{"month", "%m", "03"},
		{"day", "%d", "22"},
		{"hour", "%H", "14"},
		{"minute", "%M", "30"},
		{"second", "%S", "05"},
		{"time", "%T", "14:30:05"},
		{"literal percent", "%%", "%"},
		{"combined", "log-%F-%H-%M-%S.txt", "log-2026-03-22-14-30-05.txt"},
		{"no tokens", "plain.txt", "plain.txt"},
		{"unknown token passthrough", "%Z-thing", "%Z-thing"},
		{"adjacent tokens", "%Y%m%d", "20260322"},
		{"trailing percent", "file%", "file%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandStrftimeAt(tt.input, ts)
			if got != tt.expect {
				t.Errorf("expandStrftimeAt(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	home := "/Users/matt"
	tests := []struct {
		input  string
		expect string
	}{
		{"~/file.log", home + "/file.log"},
		{"~/", home + "/"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~other", "~other"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandTildeWith(tt.input, home)
			if got != tt.expect {
				t.Errorf("expandTildeWith(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
