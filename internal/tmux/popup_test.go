package tmux

import (
	"reflect"
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

func TestResolvePopupStyleReadsPluginOptions(t *testing.T) {
	fake := &fakeClient{commandOutput: "@tmux-popup-control-popup-border-lines=heavy\n" +
		"@tmux-popup-control-popup-border-style=fg=red,bold\n"}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	// the popup style is tmux-side configuration: the run-shell -C launcher
	// can only see tmux options, so env vars must not change it here either.
	t.Setenv("TMUX_POPUP_CONTROL_POPUP_BORDER_LINES", "rounded")

	want := PopupStyle{BorderLines: "heavy", BorderStyle: "fg=red,bold"}
	if got := ResolvePopupStyle(""); got != want {
		t.Fatalf("unexpected style\n got: %+v\nwant: %+v", got, want)
	}
}
