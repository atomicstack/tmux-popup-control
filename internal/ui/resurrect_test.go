package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

// TestResurrectProgressFlow verifies that progress messages accumulate log
// entries and update step/total on the resurrectState.
func TestResurrectProgressFlow(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	ch := make(chan resurrect.ProgressEvent, 5)
	m.resurrectState = &resurrectState{
		operation: "save",
		progress:  ch,
	}

	events := []resurrect.ProgressEvent{
		{Step: 1, Total: 3, Message: "saving session 'dev'...", Kind: "session", ID: "dev"},
		{Step: 2, Total: 3, Message: "saving window 'editor'", Kind: "window", ID: "editor"},
		{Step: 3, Total: 3, Message: "saving pane 0", Kind: "pane", ID: "0"},
	}

	model := tea.Model(m)
	for _, evt := range events {
		model, _ = model.(*Model).Update(resurrectProgressMsg{event: evt})
	}

	s := model.(*Model).resurrectState
	if s == nil {
		t.Fatal("expected resurrectState to be non-nil")
	}
	if s.step != 3 {
		t.Fatalf("expected step=3, got %d", s.step)
	}
	if s.total != 3 {
		t.Fatalf("expected total=3, got %d", s.total)
	}
	if len(s.log) != 3 {
		t.Fatalf("expected 3 log entries, got %d", len(s.log))
	}
	if s.log[0].message != "saving session 'dev'..." {
		t.Fatalf("unexpected first log entry: %q", s.log[0].message)
	}
	if s.log[0].kind != "session" {
		t.Fatalf("expected kind=session, got %q", s.log[0].kind)
	}
	if s.log[1].kind != "window" {
		t.Fatalf("expected kind=window, got %q", s.log[1].kind)
	}
	if s.log[2].kind != "pane" {
		t.Fatalf("expected kind=pane, got %q", s.log[2].kind)
	}
}

// TestResurrectProgressNoMessageSkipsLogEntry verifies that progress events
// with empty messages do not add log entries.
func TestResurrectProgressNoMessageSkipsLogEntry(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	ch := make(chan resurrect.ProgressEvent, 5)
	m.resurrectState = &resurrectState{
		operation: "save",
		progress:  ch,
	}

	// event with no message
	mdl, _ := m.Update(resurrectProgressMsg{event: resurrect.ProgressEvent{Step: 1, Total: 5}})
	s := mdl.(*Model).resurrectState
	if len(s.log) != 0 {
		t.Fatalf("expected 0 log entries for empty-message event, got %d", len(s.log))
	}
}

// TestResurrectKeysDuringProgress verifies that key presses are consumed
// while the operation is still running (done=false).
func TestResurrectKeysDuringProgress(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	m.resurrectState = &resurrectState{
		operation: "save",
		done:      false,
	}

	h := NewHarness(m)
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if h.Model().mode != ModeResurrect {
		t.Fatalf("expected mode to stay ModeResurrect, got %d", h.Model().mode)
	}
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if h.Model().mode != ModeResurrect {
		t.Fatalf("expected mode to stay ModeResurrect after escape, got %d", h.Model().mode)
	}
}

// TestResurrectKeyOnError verifies that a key press on a failed operation
// dismisses the resurrect UI and returns to the menu.
func TestResurrectKeyOnError(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	m.resurrectState = &resurrectState{
		operation: "save",
		done:      true,
		err:       errors.New("save failed"),
	}

	h := NewHarness(m)
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if h.Model().mode != ModeMenu {
		t.Fatalf("expected mode to become ModeMenu, got %d", h.Model().mode)
	}
	if h.Model().resurrectState != nil {
		t.Fatalf("expected resurrectState to be cleared after error dismiss")
	}
}

// TestResurrectKeyOnSuccess verifies that a key press on a successful
// operation emits tea.Quit.
func TestResurrectKeyOnSuccess(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	m.resurrectState = &resurrectState{
		operation: "save",
		done:      true,
		err:       nil,
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command on success key press")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestResurrectViewContainsLogAndBar verifies that the resurrect view includes
// log messages and progress bar characters.
func TestResurrectViewContainsLogAndBar(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	m.resurrectState = &resurrectState{
		operation: "save",
		log: []logEntry{
			{message: "saving session 'work'...", kind: "session", id: "work"},
			{message: "saving window 'term'", kind: "window", id: "term"},
		},
		step:  2,
		total: 5,
	}

	view := m.resurrectView()

	if !strings.Contains(view, "saving session 'work'...") {
		t.Fatalf("expected session log entry in view, got:\n%s", view)
	}
	if !strings.Contains(view, "saving window 'term'") {
		t.Fatalf("expected window log entry in view, got:\n%s", view)
	}
	// progress bar should contain filled and empty characters
	if !strings.Contains(view, "█") {
		t.Fatalf("expected filled bar character in view, got:\n%s", view)
	}
	if !strings.Contains(view, "░") {
		t.Fatalf("expected empty bar character in view, got:\n%s", view)
	}
}

// TestResurrectDiscoveringPhase verifies that when no log entries and total=0
// exist, the view shows the discovering indicator.
func TestResurrectDiscoveringPhase(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	m.resurrectState = &resurrectState{
		operation: "restore",
		log:       nil,
		step:      0,
		total:     0,
	}

	view := m.resurrectView()

	if !strings.Contains(view, "discovering...") {
		t.Fatalf("expected 'discovering...' in view during empty phase, got:\n%s", view)
	}
}

// TestResurrectDoneEventSetsFlag verifies that a progress event with Done=true
// transitions the state to done.
func TestResurrectDoneEventSetsFlag(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	ch := make(chan resurrect.ProgressEvent, 1)
	m.resurrectState = &resurrectState{
		operation: "save",
		progress:  ch,
	}

	doneEvt := resurrect.ProgressEvent{
		Step:    5,
		Total:   5,
		Message: "done",
		Kind:    "info",
		Done:    true,
	}

	mdl, _ := m.Update(resurrectProgressMsg{event: doneEvt})
	s := mdl.(*Model).resurrectState
	if !s.done {
		t.Fatal("expected done=true after done event")
	}
	if s.err != nil {
		t.Fatalf("expected no error, got %v", s.err)
	}
}

// TestResurrectDoneErrorEventSetsError verifies that a progress event with
// Done=true and a non-nil Err records the error correctly.
func TestResurrectDoneErrorEventSetsError(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	ch := make(chan resurrect.ProgressEvent, 1)
	m.resurrectState = &resurrectState{
		operation: "save",
		progress:  ch,
	}

	errEvt := resurrect.ProgressEvent{
		Step:  2,
		Total: 5,
		Done:  true,
		Err:   errors.New("permission denied"),
	}

	mdl, cmd := m.Update(resurrectProgressMsg{event: errEvt})
	s := mdl.(*Model).resurrectState
	if !s.done {
		t.Fatal("expected done=true")
	}
	if s.err == nil {
		t.Fatal("expected non-nil error")
	}
	if s.err.Error() != "permission denied" {
		t.Fatalf("unexpected error message: %v", s.err)
	}
	// on error, no tick command should be returned
	if cmd != nil {
		t.Fatalf("expected no command on error completion, got %T", cmd)
	}
}

// TestResurrectViewRestoreOperation verifies the restore operation also
// renders the progress bar (gradient direction differs but bar chars present).
func TestResurrectViewRestoreOperation(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.mode = ModeResurrect
	m.resurrectState = &resurrectState{
		operation: "restore",
		log: []logEntry{
			{message: "restoring session 'main'...", kind: "session", id: "main"},
		},
		step:  3,
		total: 10,
	}

	view := m.resurrectView()

	if !strings.Contains(view, "restoring session 'main'...") {
		t.Fatalf("expected restore log in view, got:\n%s", view)
	}
	if !strings.Contains(view, "█") {
		t.Fatalf("expected filled bar character in restore view, got:\n%s", view)
	}
	if !strings.Contains(view, "░") {
		t.Fatalf("expected empty bar character in restore view, got:\n%s", view)
	}
}
