package ui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWithPromptResetsStateAndReturnsCommand(t *testing.T) {
	m := NewModel("", 0, 0, false, true, nil, "")
	m.loading = true
	m.pendingID = "test"
	m.pendingLabel = "label"
	m.errMsg = "previous"
	m.setInfo("old info")

	cmd := m.withPrompt(func() promptResult {
		return promptResult{Cmd: tea.Quit, Info: "executed"}
	})

	if m.loading {
		t.Fatalf("expected loading cleared")
	}
	if m.pendingID != "" || m.pendingLabel != "" {
		t.Fatalf("expected pending fields cleared, got %q %q", m.pendingID, m.pendingLabel)
	}
	if m.errMsg != "" {
		t.Fatalf("expected error cleared, got %q", m.errMsg)
	}
	if m.infoMsg != "executed" {
		t.Fatalf("expected info message set, got %q", m.infoMsg)
	}
	if cmd == nil {
		t.Fatalf("expected command returned")
	}
	if msg := cmd(); msg == nil {
		t.Fatalf("expected command to emit a message")
	} else if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected quit message, got %T", msg)
	}
}

func TestWithPromptHandlesError(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "")
	boom := errors.New("boom")

	cmd := m.withPrompt(func() promptResult {
		return promptResult{Err: boom}
	})

	if cmd != nil {
		t.Fatalf("expected no command on error")
	}
	if m.errMsg != boom.Error() {
		t.Fatalf("expected error message %q, got %q", boom.Error(), m.errMsg)
	}
	if m.infoMsg != "" {
		t.Fatalf("expected info cleared on error, got %q", m.infoMsg)
	}
}
