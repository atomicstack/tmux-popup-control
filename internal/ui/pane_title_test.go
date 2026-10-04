package ui

import (
	"strings"
	"testing"
)

// TestPaneTitleFollowsBreadcrumb pins that the popup pane's title carries the
// breadcrumb (which the view no longer draws), is sent only when it changes,
// and is never sent when the binary is not running as a popup.
func TestPaneTitleFollowsBreadcrumb(t *testing.T) {
	var titles []string
	orig := setPaneTitleFn
	setPaneTitleFn = func(_, pane, title string) error { titles = append(titles, pane+"="+title); return nil }
	t.Cleanup(func() { setPaneTitleFn = orig })

	t.Setenv("TMUX_PANE", "%9")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", "%1")
	m := NewModel(ModelConfig{})

	if cmd := m.syncPaneTitleCmd(); cmd != nil {
		t.Fatalf("root title is set by new-pane -T; nothing should be sent")
	}
	m.stack = append(m.stack, newLevel("session", "session", nil, nil))
	m.stack = append(m.stack, newLevel("session:switch", "switch", nil, nil))
	cmd := m.syncPaneTitleCmd()
	if cmd == nil {
		t.Fatalf("expected a retitle after navigating")
	}
	cmd()
	if cmd := m.syncPaneTitleCmd(); cmd != nil {
		t.Fatalf("unchanged title must not be resent")
	}
	m.stack = m.stack[:1]
	if cmd := m.syncPaneTitleCmd(); cmd != nil {
		cmd()
	}
	want := []string{"%9=tmux-popup-control: session→switch", "%9=tmux-popup-control"}
	if strings.Join(titles, "|") != strings.Join(want, "|") {
		t.Fatalf("titles = %q, want %q", titles, want)
	}

	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", "")
	plain := NewModel(ModelConfig{})
	plain.stack = append(plain.stack, newLevel("session", "session", nil, nil))
	if cmd := plain.syncPaneTitleCmd(); cmd != nil {
		t.Fatalf("outside a popup the pane must never be retitled")
	}
}
