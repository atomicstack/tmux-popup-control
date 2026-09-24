package tmux

import (
	"reflect"
	"strings"
	"testing"
)

func TestPopupStyleArgsEmptyAddsNoFlags(t *testing.T) {
	if args := (PopupStyle{}).Args(); len(args) != 0 {
		t.Fatalf("expected no flags for an empty style, got %q", args)
	}
}

func TestPopupStyleArgsMapsEachField(t *testing.T) {
	style := PopupStyle{
		BorderLines: "rounded",
		BorderStyle: "fg=colour239",
		Style:       "bg=terminal,fg=terminal",
	}
	want := []string{
		"-B", "rounded",
		"-S", "fg=colour239",
		"-R", "fg=colour239",
		"-s", "bg=terminal,fg=terminal",
	}
	if got := style.Args(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected flags\n got: %q\nwant: %q", got, want)
	}
}

func TestPopupStyleArgsOmitsUnsetFields(t *testing.T) {
	want := []string{"-B", "double"}
	if got := (PopupStyle{BorderLines: "double"}).Args(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected flags\n got: %q\nwant: %q", got, want)
	}
}

func TestResolvePopupStyleReadsOptionsAndPrefersEnv(t *testing.T) {
	withStubCommander(t, func(name string, args ...string) commander {
		joined := strings.Join(args, " ")
		switch {
		case strings.HasSuffix(joined, "@tmux-popup-control-popup-border-lines"):
			return stubCommander{output: []byte("heavy\n")}
		case strings.HasSuffix(joined, "@tmux-popup-control-popup-border-style"):
			return stubCommander{output: []byte("fg=red\n")}
		}
		return stubCommander{output: nil}
	})
	t.Setenv("TMUX_POPUP_CONTROL_POPUP_BORDER_LINES", "rounded")
	t.Setenv("TMUX_POPUP_CONTROL_POPUP_BORDER_STYLE", "")
	t.Setenv("TMUX_POPUP_CONTROL_POPUP_STYLE", "")

	want := PopupStyle{BorderLines: "rounded", BorderStyle: "fg=red"}
	if got := ResolvePopupStyle(""); got != want {
		t.Fatalf("unexpected style\n got: %+v\nwant: %+v", got, want)
	}
}
