package ui

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestEnsurePreviewForLevelSchedulesCommand(t *testing.T) {
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m := NewModel("", 0, 0, false, false, nil, "", "")
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.windows.SetEntries([]menu.WindowEntry{{Session: "dev", Index: 1, Name: "main"}})

	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatalf("expected preview command")
	}
	msg := cmd()
	previewMsg, ok := msg.(previewLoadedMsg)
	if !ok {
		t.Fatalf("expected previewLoadedMsg, got %T", msg)
	}
	m.handlePreviewLoadedMsg(previewMsg)
	data := m.activePreview()
	if data == nil {
		t.Fatalf("expected preview data to be populated")
	}
	if len(data.lines) == 0 {
		t.Fatalf("expected preview lines, got %#v", data.lines)
	}
	if data.loading {
		t.Fatalf("expected loading to be false")
	}
}

func TestHandlePreviewLoadedMsgIgnoresStaleResponses(t *testing.T) {
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m := &Model{
		stack: []*level{lvl},
		preview: map[string]*previewData{
			"session:switch": {target: "dev", seq: 2},
		},
	}
	msg := previewLoadedMsg{
		levelID: "session:switch",
		target:  "dev",
		seq:     1,
		lines:   []string{"old"},
	}
	m.handlePreviewLoadedMsg(msg)
	data := m.activePreview()
	if data.lines != nil {
		t.Fatalf("expected stale message to be ignored, got %+v", data)
	}
}
