package testutil

import "testing"

func TestNormalizeCursorBlink(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			"cursor on opening paren",
			"\x1b[38;5;241m\x1b[48;5;33m(\x1b[49mtype to search)\x1b[39m",
			"\x1b[38;5;241m(type to search)\x1b[39m",
		},
		{
			"no cursor present",
			"\x1b[38;5;241m(type to search)\x1b[39m",
			"\x1b[38;5;241m(type to search)\x1b[39m",
		},
		{
			"multi-char bg not stripped",
			"\x1b[48;5;238m session\x1b[0m",
			"\x1b[48;5;238m session\x1b[0m",
		},
		{
			"cursor on different char",
			"hello \x1b[48;5;12mw\x1b[49morld",
			"hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCursorBlink(tt.input)
			if got != tt.expect {
				t.Errorf("normalizeCursorBlink()\n got: %q\nwant: %q", got, tt.expect)
			}
		})
	}
}
