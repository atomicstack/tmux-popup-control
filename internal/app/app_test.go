package app

import (
	"errors"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func stubResurrectFns(t *testing.T) {
	t.Helper()

	oldPaneContents := resolvePaneContentsFn
	oldResolveDir := resolveSaveDirFn
	oldLatestSave := latestSaveFn
	resolvePaneContentsFn = func(string) bool { return false }
	resolveSaveDirFn = func(string) (string, error) { return "", nil }
	latestSaveFn = func(string) (string, error) { return "", nil }
	t.Cleanup(func() {
		resolvePaneContentsFn = oldPaneContents
		resolveSaveDirFn = oldResolveDir
		latestSaveFn = oldLatestSave
	})
}

func TestColorProfileOverrideRecognizesAliases(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  colorprofile.Profile
		ok    bool
	}{
		{name: "empty", value: "", want: colorprofile.Profile(0), ok: false},
		{name: "truecolor", value: "24-bit", want: colorprofile.TrueColor, ok: true},
		{name: "ansi256", value: "256color", want: colorprofile.ANSI256, ok: true},
		{name: "ansi", value: "ansi", want: colorprofile.ANSI, ok: true},
		{name: "ascii", value: "ascii", want: colorprofile.ASCII, ok: true},
		{name: "unknown", value: "bogus", want: colorprofile.Profile(0), ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TMUX_POPUP_CONTROL_COLOR_PROFILE", tt.value)
			got, ok := colorProfileOverride()
			if got != tt.want || ok != tt.ok {
				t.Fatalf("colorProfileOverride() = (%v, %v), want (%v, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestBuildResurrectStartPrefersExplicitSaveDirAndFrom(t *testing.T) {
	stubResurrectFns(t)
	resolvePaneContentsFn = func(string) bool { return false }
	resolveSaveDirFn = func(string) (string, error) {
		t.Fatal("ResolveDir should not be called when SessionStorageDir is set")
		return "", nil
	}
	latestSaveFn = func(string) (string, error) {
		t.Fatal("LatestSave should not be called when ResurrectFrom is set")
		return "", nil
	}

	start := buildResurrectStart(Config{
		ResurrectOp:         "restore",
		ResurrectName:       "snap",
		ResurrectFrom:       "/tmp/explicit.json",
		SessionStorageDir:   "/tmp/saves",
		RestorePaneContents: true,
	}, "test.sock", "client-1")

	if start.Config.SaveDir != "/tmp/saves" {
		t.Fatalf("expected save dir /tmp/saves, got %q", start.Config.SaveDir)
	}
	if start.SaveFile != "/tmp/explicit.json" {
		t.Fatalf("expected explicit save file, got %q", start.SaveFile)
	}
	if !start.Config.CapturePaneContents {
		t.Fatal("expected explicit RestorePaneContents to win")
	}
}

func TestBuildResurrectStartFallsBackToResolvedDirAndLatestSave(t *testing.T) {
	stubResurrectFns(t)
	resolvePaneContentsFn = func(string) bool { return true }
	resolveSaveDirFn = func(string) (string, error) { return "/tmp/saves", nil }
	latestSaveFn = func(dir string) (string, error) { return dir + "/latest.json", nil }

	start := buildResurrectStart(Config{
		ResurrectOp: "restore",
	}, "test.sock", "client-1")

	if start.Config.SaveDir != "/tmp/saves" {
		t.Fatalf("expected resolved save dir, got %q", start.Config.SaveDir)
	}
	if start.SaveFile != "/tmp/saves/latest.json" {
		t.Fatalf("expected latest save path, got %q", start.SaveFile)
	}
	if !start.Config.CapturePaneContents {
		t.Fatal("expected resolved pane-contents setting to be used")
	}
}

func TestBuildResurrectStartIgnoresResolveDirErrors(t *testing.T) {
	stubResurrectFns(t)
	resolveSaveDirFn = func(string) (string, error) { return "", errors.New("boom") }
	latestSaveFn = func(string) (string, error) {
		t.Fatal("LatestSave should not run without a save dir")
		return "", nil
	}

	start := buildResurrectStart(Config{
		ResurrectOp: "restore",
	}, "test.sock", "client-1")

	if start.Config.SaveDir != "" {
		t.Fatalf("expected empty save dir, got %q", start.Config.SaveDir)
	}
	if start.SaveFile != "" {
		t.Fatalf("expected empty save file, got %q", start.SaveFile)
	}
}
