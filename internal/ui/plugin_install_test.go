package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

func TestPluginInstallViewPinsGradientBarToBottomRow(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 10})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{plugin: plugin.Plugin{Name: "alpha"}, status: pluginInstallCloning},
			{plugin: plugin.Plugin{Name: "beta"}, status: pluginInstallQueued},
		},
		operation:       "install",
		progressCurrent: 2,
		progressTotal:   8,
	}

	view := m.pluginInstallView()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	last := lastNonEmptyLine(lines)
	if !strings.Contains(last, "█") {
		t.Fatalf("expected bottom row to contain the progress bar, got last row %q in:\n%s", last, view)
	}
	if !strings.Contains(view, "\x1b[38;2;") {
		t.Fatalf("expected gradient truecolor output in progress bar, got:\n%s", view)
	}
	if strings.Contains(last, "alpha") || strings.Contains(last, "beta") {
		t.Fatalf("expected plugin names to stay out of the bottom bar row, got last row %q in:\n%s", last, view)
	}
}

func TestPluginInstallViewLeavesSummaryAboveBottomBar(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 10})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{plugin: plugin.Plugin{Name: "alpha"}, status: pluginInstallDone},
		},
		operation:       "install",
		finished:        true,
		summary:         "Installed 1 plugin(s)",
		installed:       []plugin.Plugin{{Name: "alpha"}},
		progressCurrent: 4,
		progressTotal:   4,
	}

	view := m.pluginInstallView()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	last := lastNonEmptyLine(lines)
	if !strings.Contains(last, "█") {
		t.Fatalf("expected bottom row to remain the progress bar after completion, got last row %q in:\n%s", last, view)
	}
	if !strings.Contains(view, "Reload plugins? [y/n]") {
		t.Fatalf("expected reload prompt in finished view, got:\n%s", view)
	}
	if idx := strings.LastIndex(view, "Reload plugins? [y/n]"); idx > strings.LastIndex(view, "█") {
		t.Fatalf("expected reload prompt to stay above the bar, got:\n%s", view)
	}
}

func TestPluginInstallViewShowsPhaseLabelsAndInlineFailure(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 44})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{plugin: plugin.Plugin{Name: "queued"}, status: pluginInstallQueued},
			{plugin: plugin.Plugin{Name: "prep"}, status: pluginInstallPreparing},
			{plugin: plugin.Plugin{Name: "clone"}, status: pluginInstallCloning},
			{plugin: plugin.Plugin{Name: "pull"}, status: pluginInstallPulling},
			{plugin: plugin.Plugin{Name: "mods"}, status: pluginInstallSubmodules},
			{plugin: plugin.Plugin{Name: "done"}, status: pluginInstallDone},
			{plugin: plugin.Plugin{Name: "oops"}, status: pluginInstallFailed, err: errors.New("boom: something exploded")},
		},
		operation:       "update",
		progressCurrent: 7,
		progressTotal:   30,
	}

	view := m.pluginInstallView()
	for _, want := range []string{"queued", "preparing", "cloning", "pulling", "submodules", "done", "failed"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q label in view, got:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "boom") {
		t.Fatalf("expected short inline failure text in view, got:\n%s", view)
	}
}

func TestPluginInstallViewRendersMockupStyleCells(t *testing.T) {
	m := NewModel(ModelConfig{Width: 88, Height: 14})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{
				plugin: plugin.Plugin{
					Name:   "alpha",
					Source: "tmux-plugins/tmux-alpha",
					Dir:    "/tmp/plugins/alpha",
				},
				status: pluginInstallCloning,
			},
			{
				plugin: plugin.Plugin{
					Name:   "beta",
					Source: "tmux-plugins/tmux-beta",
					Dir:    "/tmp/plugins/beta",
				},
				status: pluginInstallFailed,
				err:    errors.New("git pull failed: remote refused the update"),
			},
		},
		operation:       "install",
		progressCurrent: 3,
		progressTotal:   8,
	}

	view := m.pluginInstallView()
	if !strings.Contains(view, "┌") || !strings.Contains(view, "│") {
		t.Fatalf("expected straight-corner bordered cells, got:\n%s", view)
	}
	if !strings.Contains(view, "remote refused") {
		t.Fatalf("expected short inline failure text in cell view, got:\n%s", view)
	}

	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected cell lines in view, got:\n%s", view)
	}

	nameLine := stripANSI(lines[1])
	sourceLine := stripANSI(lines[2])
	if !strings.Contains(nameLine, "alpha") {
		t.Fatalf("expected plugin name on first content row, got %q in:\n%s", nameLine, view)
	}
	if strings.Contains(nameLine, "cloning") {
		t.Fatalf("expected status to move off the first content row, got %q in:\n%s", nameLine, view)
	}
	if !strings.Contains(sourceLine, "tmux-plugins/tmux-alpha -> /tmp/plugins/alpha/") {
		t.Fatalf("expected source arrow destination on second content row, got %q in:\n%s", sourceLine, view)
	}
	if !strings.Contains(sourceLine, "cloning") {
		t.Fatalf("expected status on second content row, got %q in:\n%s", sourceLine, view)
	}
}

// TestPluginInstallViewUninstallAskingPhaseRendersInline verifies the
// per-plugin y/n prompt now renders inside the bordered cell layout —
// the entry currently being asked shows "asking" with the prompt text
// in the detail row, prior answers stay visible as accepted/skipped,
// and unanswered entries show as queued for confirmation.
func TestPluginInstallViewUninstallAskingPhaseRendersInline(t *testing.T) {
	m := NewModel(ModelConfig{Width: 88, Height: 28})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{plugin: plugin.Plugin{Name: "alpha", Dir: "/tmp/plugins/alpha"}, status: pluginInstallAccepted},
			{plugin: plugin.Plugin{Name: "beta", Dir: "/tmp/plugins/beta"}, status: pluginInstallSkipped},
			{plugin: plugin.Plugin{Name: "gamma", Dir: "/tmp/plugins/gamma"}, status: pluginInstallAsking},
			{plugin: plugin.Plugin{Name: "delta", Dir: "/tmp/plugins/delta"}, status: pluginInstallQueued},
		},
		operation:       "uninstall",
		progressCurrent: 3,
		progressTotal:   18,
	}

	view := m.pluginInstallView()
	if !strings.Contains(view, "┌") || !strings.Contains(view, "│") {
		t.Fatalf("expected bordered cells in asking phase, got:\n%s", view)
	}
	for _, want := range []string{"alpha", "beta", "gamma", "delta"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected plugin %q to be visible during asking, got:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "pending removal") {
		t.Fatalf("expected 'pending removal' status pill for alpha, got:\n%s", view)
	}
	if !strings.Contains(view, "skipped") {
		t.Fatalf("expected 'skipped' status pill for beta, got:\n%s", view)
	}
	if !strings.Contains(view, "asking") {
		t.Fatalf("expected 'asking' status pill for gamma, got:\n%s", view)
	}
	if !strings.Contains(view, "awaiting confirmation") {
		t.Fatalf("expected asking cell detail row to read 'awaiting confirmation', got:\n%s", view)
	}
	if !strings.Contains(view, "waiting for confirmation") {
		t.Fatalf("expected delta to show 'waiting for confirmation', got:\n%s", view)
	}

	// The y/n prompt for the asking entry must live on its own line
	// directly above the bottom progress bar — not inside the cell.
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 rendered rows, got:\n%s", view)
	}
	bottomRow := stripANSI(lines[len(lines)-1])
	promptRow := stripANSI(lines[len(lines)-2])
	if !strings.Contains(promptRow, "remove gamma (/tmp/plugins/gamma)? [y/n]") {
		t.Fatalf("expected prompt above progress bar to ask about gamma, got %q in:\n%s", promptRow, view)
	}
	// Crude sanity check that the bottom row is the progress bar (it
	// includes the "N/M" counter rendered by buildPluginInstallProgressBar).
	if !strings.Contains(bottomRow, "/") {
		t.Fatalf("expected bottom row to be the progress bar, got %q", bottomRow)
	}
}

// TestPluginInstallViewUninstallRendersMatchingCells verifies the uninstall
// progress display reuses the same bordered-cell layout as install/update,
// with uninstall-flavored phase labels and detail text.
func TestPluginInstallViewUninstallRendersMatchingCells(t *testing.T) {
	m := NewModel(ModelConfig{Width: 88, Height: 14})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{
				plugin: plugin.Plugin{
					Name: "alpha",
					Dir:  "/tmp/plugins/alpha",
				},
				status: pluginInstallRemoving,
			},
			{
				plugin: plugin.Plugin{
					Name: "beta",
					Dir:  "/tmp/plugins/beta",
				},
				status: pluginInstallFailed,
				err:    errors.New("permission denied"),
			},
		},
		operation:       "uninstall",
		progressCurrent: 3,
		progressTotal:   8,
	}

	view := m.pluginInstallView()
	if !strings.Contains(view, "┌") || !strings.Contains(view, "│") {
		t.Fatalf("expected bordered cells for uninstall view, got:\n%s", view)
	}
	if !strings.Contains(view, "removing") {
		t.Fatalf("expected uninstall phase label 'removing' in view, got:\n%s", view)
	}
	if !strings.Contains(view, "removing plugin directory") {
		t.Fatalf("expected uninstall detail text in view, got:\n%s", view)
	}
	if !strings.Contains(view, "permission denied") {
		t.Fatalf("expected per-plugin failure text in view, got:\n%s", view)
	}
}

// TestPluginInstallViewUninstallDoneShowsReloadPrompt verifies the
// "Reload plugins?" prompt is presented after uninstall completes (the
// prompt was previously emitted from pluginConfirmView).
func TestPluginInstallViewUninstallDoneShowsReloadPrompt(t *testing.T) {
	m := NewModel(ModelConfig{Width: 88, Height: 14})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{
				plugin: plugin.Plugin{Name: "alpha", Dir: "/tmp/plugins/alpha"},
				status: pluginInstallDone,
			},
		},
		operation:       "uninstall",
		finished:        true,
		installed:       []plugin.Plugin{{Name: "alpha", Dir: "/tmp/plugins/alpha"}},
		summary:         "Uninstalled 1 plugin(s)",
		progressCurrent: 5,
		progressTotal:   5,
	}

	view := m.pluginInstallView()
	if !strings.Contains(view, "Uninstalled 1 plugin(s)") {
		t.Fatalf("expected uninstall summary in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Reload plugins? [y/n]") {
		t.Fatalf("expected reload prompt after uninstall, got:\n%s", view)
	}
}

func TestPluginInstallViewUsesDimSideBorders(t *testing.T) {
	lines := pluginInstallEntryCellLines(pluginInstallEntry{
		plugin: plugin.Plugin{
			Name:   "alpha",
			Source: "github.com/atomicstack/maccyakto",
			Dir:    "/tmp/plugins/maccyakto",
		},
		status: pluginInstallCloning,
	}, 72, "install")
	if len(lines) < 4 {
		t.Fatalf("expected cell lines, got %#v", lines)
	}

	const borderANSI = "\x1b[38;5;238;48;2;28;28;28m│"
	if count := strings.Count(lines[1].text, borderANSI); count != 2 {
		t.Fatalf("expected both side borders on name row to use dim border style, got count=%d in %q", count, lines[1].text)
	}
	if count := strings.Count(lines[2].text, borderANSI); count != 2 {
		t.Fatalf("expected both side borders on source row to use dim border style, got count=%d in %q", count, lines[2].text)
	}
}

func TestPluginInstallViewKeepsBackgroundAcrossPaddedInterior(t *testing.T) {
	lines := pluginInstallEntryCellLines(pluginInstallEntry{
		plugin: plugin.Plugin{
			Name:   "alpha",
			Source: "github.com/atomicstack/maccyakto",
			Dir:    "/tmp/plugins/maccyakto",
		},
		status: pluginInstallCloning,
	}, 72, "install")
	if len(lines) < 4 {
		t.Fatalf("expected cell lines, got %#v", lines)
	}

	const bgSpaceANSI = "\x1b[48;2;28;28;28m "
	if !strings.Contains(lines[2].text, bgSpaceANSI) {
		t.Fatalf("expected padded source row spaces to keep cell background, got %q", lines[2].text)
	}
	if !strings.Contains(lines[3].text, bgSpaceANSI) {
		t.Fatalf("expected padded detail row spaces to keep cell background, got %q", lines[3].text)
	}
}

func TestPluginInstallDisplayPathNormalizesEnvPrefixes(t *testing.T) {
	t.Setenv("HOME", "/Users/matt")
	t.Setenv("TMPDIR", "/private/var/folders/abc/T/")
	t.Setenv("TMP", "/tmp/custom")

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "tmpdir prefix",
			in:   "/private/var/folders/abc/T/plugin-cache/foo",
			want: "$TMPDIR/plugin-cache/foo/",
		},
		{
			name: "tmp prefix",
			in:   "/tmp/custom/plugin-cache/foo",
			want: "$TMP/plugin-cache/foo/",
		},
		{
			name: "home prefix",
			in:   "/Users/matt/.tmux/plugins/foo",
			want: "~/.tmux/plugins/foo/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pluginInstallDisplayPath(tt.in); got != tt.want {
				t.Fatalf("pluginInstallDisplayPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPluginInstallFinishedViewKeepsSummaryVisibleWithLongList(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 8})
	m.mode = ModePluginInstall
	entries := make([]pluginInstallEntry, 12)
	for i := range entries {
		entries[i] = pluginInstallEntry{
			plugin: plugin.Plugin{Name: "plugin-" + strings.Repeat("x", i)},
			status: pluginInstallDone,
		}
	}
	m.pluginInstallState = &pluginInstallState{
		entries:         entries,
		operation:       "install",
		finished:        true,
		installed:       []plugin.Plugin{{Name: "plugin-0"}},
		summary:         "Installed 1 plugin(s)",
		progressCurrent: 10,
		progressTotal:   10,
	}

	view := m.pluginInstallView()
	if !strings.Contains(view, "Installed 1 plugin(s)") {
		t.Fatalf("expected finished summary to remain visible, got:\n%s", view)
	}
	if !strings.Contains(view, "Reload plugins? [y/n]") {
		t.Fatalf("expected reload prompt to remain visible, got:\n%s", view)
	}
}

func TestPluginInstallFinishedInstallReloadUsesSourceablePlugins(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 8})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{
				plugin: plugin.Plugin{Name: "alpha", Source: "tmux-plugins/tmux-alpha", Dir: "/tmp/plugins/alpha"},
				status: pluginInstallDone,
			},
		},
		operation:       "install",
		progressCurrent: 3,
		progressTotal:   4,
	}
	m.finishPluginInstall()
	if !m.pluginInstallState.finished {
		t.Fatal("expected finished state after finalizing install")
	}
	if len(m.pluginInstallState.installed) != 1 {
		t.Fatalf("expected one installed plugin, got %d", len(m.pluginInstallState.installed))
	}
	if !m.pluginInstallState.installed[0].Installed {
		t.Fatal("expected finished install list to contain sourceable plugin copies")
	}

	var got []plugin.Plugin
	origReload := reloadPluginsFn
	reloadPluginsFn = func(_ string, plugins []plugin.Plugin) error {
		got = append(got, plugins...)
		return nil
	}
	t.Cleanup(func() { reloadPluginsFn = origReload })

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("expected reload command after y")
	}
	msg := cmd()
	if _, ok := msg.(menu.ActionResult); !ok {
		t.Fatalf("expected menu.ActionResult, got %T", msg)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 plugin passed to reload, got %d", len(got))
	}
	if !got[0].Installed {
		t.Fatal("expected reloaded plugin to be marked Installed")
	}
	if got[0].Dir == "" {
		t.Fatal("expected reloaded plugin to have a directory")
	}
}

func TestPluginInstallInstallPhasesAdvanceProgress(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 12})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{plugin: plugin.Plugin{Name: "alpha"}, status: pluginInstallQueued},
		},
		operation:       "install",
		progressCurrent: 1,
		progressTotal:   pluginInstallTotalSteps("install", 1),
	}

	cmd := m.advancePluginInstall()
	if got := m.pluginInstallState.progressCurrent; got != 2 {
		t.Fatalf("expected progressCurrent=2 after preparing, got %d", got)
	}
	stage := mustPluginInstallStageMsg(t, cmd)
	if stage.phase != pluginInstallCloning {
		t.Fatalf("expected cloning stage, got %v", stage.phase)
	}
	m.handlePluginInstallStageMsg(stage)
	if got := m.pluginInstallState.progressCurrent; got != 3 {
		t.Fatalf("expected progressCurrent=3 after cloning stage, got %d", got)
	}
	if got := m.pluginInstallState.entries[0].status; got != pluginInstallCloning {
		t.Fatalf("expected entry status cloning, got %v", got)
	}

	m.handlePluginInstallResultMsg(pluginInstallResultMsg{index: 0, err: nil})
	if !m.pluginInstallState.finished {
		t.Fatal("expected install flow to finish after result")
	}
	if got := m.pluginInstallState.progressCurrent; got != m.pluginInstallState.progressTotal {
		t.Fatalf("expected progress to finish at total, got %d/%d", got, m.pluginInstallState.progressTotal)
	}
	if got := m.pluginInstallState.entries[0].status; got != pluginInstallDone {
		t.Fatalf("expected entry status done, got %v", got)
	}
}

func TestPluginInstallUpdatePhasesAdvanceProgress(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 12})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{plugin: plugin.Plugin{Name: "alpha"}, status: pluginInstallQueued},
		},
		operation:       "update",
		progressCurrent: 1,
		progressTotal:   pluginInstallTotalSteps("update", 1),
	}

	cmd := m.advancePluginInstall()
	stage := mustPluginInstallStageMsg(t, cmd)
	if stage.phase != pluginInstallPulling {
		t.Fatalf("expected pulling stage, got %v", stage.phase)
	}
	m.handlePluginInstallStageMsg(stage)
	if got := m.pluginInstallState.progressCurrent; got != 3 {
		t.Fatalf("expected progressCurrent=3 after pulling stage, got %d", got)
	}
	if got := m.pluginInstallState.entries[0].status; got != pluginInstallPulling {
		t.Fatalf("expected entry status pulling, got %v", got)
	}

	m.handlePluginInstallStageMsg(pluginInstallStageMsg{index: 0, phase: pluginInstallSubmodules})
	if got := m.pluginInstallState.progressCurrent; got != 4 {
		t.Fatalf("expected progressCurrent=4 after submodules stage, got %d", got)
	}
	if got := m.pluginInstallState.entries[0].status; got != pluginInstallSubmodules {
		t.Fatalf("expected entry status submodules, got %v", got)
	}

	m.handlePluginInstallResultMsg(pluginInstallResultMsg{index: 0, err: nil})
	if !m.pluginInstallState.finished {
		t.Fatal("expected update flow to finish after result")
	}
	if got := m.pluginInstallState.progressCurrent; got != m.pluginInstallState.progressTotal {
		t.Fatalf("expected progress to finish at total, got %d/%d", got, m.pluginInstallState.progressTotal)
	}
	if got := m.pluginInstallState.entries[0].status; got != pluginInstallDone {
		t.Fatalf("expected entry status done, got %v", got)
	}
}

func TestPluginInstallUpdateFailureFinishesProgress(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 12})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{plugin: plugin.Plugin{Name: "alpha"}, status: pluginInstallQueued},
		},
		operation:       "update",
		progressCurrent: 1,
		progressTotal:   pluginInstallTotalSteps("update", 1),
	}

	stage := mustPluginInstallStageMsg(t, m.advancePluginInstall())
	if stage.phase != pluginInstallPulling {
		t.Fatalf("expected pulling stage, got %v", stage.phase)
	}
	m.handlePluginInstallStageMsg(stage)
	m.handlePluginInstallResultMsg(pluginInstallResultMsg{index: 0, err: errors.New("pull failed")})

	if !m.pluginInstallState.finished {
		t.Fatal("expected update flow to finish after pull failure")
	}
	if got := m.pluginInstallState.progressCurrent; got != m.pluginInstallState.progressTotal {
		t.Fatalf("expected failure path to finish at total, got %d/%d", got, m.pluginInstallState.progressTotal)
	}
	if got := m.pluginInstallState.entries[0].status; got != pluginInstallFailed {
		t.Fatalf("expected entry status failed, got %v", got)
	}
}

func TestPluginInstallViewClampsSmallPopupBounds(t *testing.T) {
	m := NewModel(ModelConfig{Width: 8, Height: 1})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		entries: []pluginInstallEntry{
			{plugin: plugin.Plugin{Name: "alpha"}, status: pluginInstallCloning},
		},
		operation:       "install",
		progressCurrent: 2,
		progressTotal:   8,
	}

	view := m.pluginInstallView()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > 1 {
		t.Fatalf("expected tiny popup to stay within height, got %d lines in:\n%s", len(lines), view)
	}
	for _, line := range lines {
		if visible := len([]rune(stripANSI(line))); visible > 8 {
			t.Fatalf("expected tiny popup line width <= 8, got %d in %q", visible, line)
		}
	}
}

func lastNonEmptyLine(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}

func mustPluginInstallStageMsg(t *testing.T, cmd tea.Cmd) pluginInstallStageMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	msg := cmd()
	stage, ok := msg.(pluginInstallStageMsg)
	if !ok {
		t.Fatalf("expected pluginInstallStageMsg, got %T", msg)
	}
	return stage
}

func stripANSI(s string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		case r == '\x1b':
			inEscape = true
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
