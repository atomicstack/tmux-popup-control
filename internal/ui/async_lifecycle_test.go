package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"os"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestRenamePromptsRouteToDistinctForms(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prompt tea.Msg
		mode   Mode
	}{
		{"window", menu.WindowPrompt{Target: "review:0", Initial: "original"}, ModeWindowForm},
		{"pane", menu.PanePrompt{Target: "review:0.0", Initial: "original"}, ModePaneForm},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(ModelConfig{})
			m.Update(tc.prompt)
			if m.mode != tc.mode {
				t.Fatalf("rename routed to %s; expected %s", m.mode.String(), tc.mode.String())
			}
		})
	}
}

func TestFormKeepsBackendUpdatesAlive(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.startSessionForm(menu.SessionPrompt{Action: "session:rename", Target: "original", Initial: "original"})
	m.Update(backendEventMsg{event: backend.Event{Kind: backend.KindPanes, Data: tmux.PaneSnapshot{CurrentID: "review:0.0"}}})
	if got := m.panes.CurrentID(); got != "review:0.0" {
		t.Fatalf("backend event swallowed while form active: current pane=%q", got)
	}
}

func TestFormKeepsPreviewTimerAlive(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.startSessionForm(menu.SessionPrompt{Action: "session:rename", Target: "original", Initial: "original"})
	_, cmd := m.Update(previewTickMsg{})
	if cmd == nil {
		t.Fatal("preview tick swallowed without rearming timer")
	}
}

func TestPendingSwapSurvivesBackendPoll(t *testing.T) {
	t.Run("pane", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.panes.SetEntries([]menu.PaneEntry{{ID: "review:0.0", Label: "first"}, {ID: "review:0.1", Label: "second"}})
		m.startPaneSwap(menu.PaneSwapPrompt{First: menu.Item{ID: "review:0.0"}})
		m.Update(backendEventMsg{event: backend.Event{Kind: backend.KindPanes, Data: tmux.PaneSnapshot{Panes: []tmux.Pane{{ID: "review:0.0"}, {ID: "review:0.1"}}}}})
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatalf("unchanged pane poll erased swap selection; enter does nothing, info=%q", m.infoMsg)
		}
	})
	t.Run("window", func(t *testing.T) {
		m := NewModel(ModelConfig{})
		m.windows.SetEntries([]menu.WindowEntry{{ID: "review:0", Label: "first"}, {ID: "review:1", Label: "second"}})
		m.startWindowSwap(menu.WindowSwapPrompt{First: menu.Item{ID: "review:0"}})
		m.Update(backendEventMsg{event: backend.Event{Kind: backend.KindWindows, Data: tmux.WindowSnapshot{Windows: []tmux.Window{{ID: "review:0"}, {ID: "review:1"}}}}})
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatalf("unchanged window poll erased swap selection; enter does nothing, info=%q", m.infoMsg)
		}
	})
}

func TestPreviewRefreshCoalescesInFlight(t *testing.T) {
	m := NewModel(ModelConfig{})
	lvl := newLevel("pane:switch", "panes", []menu.Item{{ID: "dev:0.0"}}, nil)
	m.stack = []*level{lvl}
	first := m.ensurePreviewForLevel(lvl)
	if first == nil {
		t.Fatal("missing first capture")
	}
	seq := m.preview[lvl.ID].seq
	if cmd := m.refreshPreviewForLevel(lvl); cmd != nil {
		t.Fatal("refresh scheduled overlapping capture")
	}
	if m.preview[lvl.ID].seq != seq {
		t.Fatal("refresh invalidated in-flight capture")
	}
}

func TestHiddenPreviewDoesNotSchedule(t *testing.T) {
	m := NewModel(ModelConfig{NoPreview: true})
	lvl := newLevel("pane:switch", "panes", []menu.Item{{ID: "dev:0.0"}}, nil)
	if cmd := m.ensurePreviewForLevel(lvl); cmd != nil {
		t.Fatal("hidden preview scheduled capture")
	}
	if cmd := m.handlePreviewTickMsg(previewTickMsg{}); cmd != nil {
		t.Fatal("hidden preview rearmed timer")
	}
}

func TestPreviewTopologyFetchedInCommand(t *testing.T) {
	previous := fetchPreviewTopologyFn
	calls := 0
	fetchPreviewTopologyFn = func(string) (tmux.PreviewTopology, error) { calls++; return tmux.PreviewTopology{}, nil }
	t.Cleanup(func() { fetchPreviewTopologyFn = previous })
	m := NewModel(ModelConfig{})
	lvl := newLevel("session:switch", "sessions", []menu.Item{{ID: "dev"}}, nil)
	cmd := m.ensurePreviewForLevel(lvl)
	if calls != 0 {
		t.Fatal("topology fetched synchronously during model update")
	}
	if cmd == nil {
		t.Fatal("missing preview command")
	}
	cmd()
	if calls != 1 {
		t.Fatal("command did not fetch topology")
	}
}

func TestLayoutQueuedPreviewCannotRunAfterEscape(t *testing.T) {
	previous := layoutPreviewFn
	var applied []string
	layoutPreviewFn = func(_, layout string) error { applied = append(applied, layout); return nil }
	t.Cleanup(func() { layoutPreviewFn = previous })
	m := NewModel(ModelConfig{})
	lvl := newLevel("window:layout", "layouts", []menu.Item{{ID: "tiled"}, {ID: "original", Label: "current layout"}}, nil)
	m.stack = append(m.stack, lvl)
	stale := m.ensurePreviewForLevel(lvl)
	restore := m.handleEscapeKey()
	if stale == nil || restore == nil {
		t.Fatal("missing layout command")
	}
	restore()
	stale()
	if len(applied) != 1 || applied[0] != "original" {
		t.Fatalf("layout changed after restore: %v", applied)
	}
}

func TestPluginResponsesIgnoreAbandonedOperation(t *testing.T) {
	for _, result := range []bool{false, true} {
		t.Run(map[bool]string{false: "stage", true: "result"}[result], func(t *testing.T) {
			m := NewModel(ModelConfig{})
			dir := t.TempDir()
			oldStage := m.startPluginProgress([]plugin.Plugin{{Name: "old", Dir: dir + "/old"}}, dir, "update")()
			stale := oldStage
			if result {
				// removal is a harmless local operation in an owned temporary directory.
				stale = m.runPluginInstallStage(0, pluginInstallRemoving)()
			}
			m.handlePluginInstallKey(tea.KeyPressMsg{Code: tea.KeyEscape})
			m.startPluginProgress([]plugin.Plugin{{Name: "new", Dir: dir + "/new"}}, dir, "update")
			before := m.pluginInstallState.entries[0].status
			if result {
				m.handlePluginInstallResultMsg(stale)
			} else {
				m.handlePluginInstallStageMsg(stale)
			}
			if m.pluginInstallState.entries[0].status != before {
				t.Fatal("old response mutated replacement operation")
			}
		})
	}
}

func TestLayoutRootEscapeRestoresBeforeQuit(t *testing.T) {
	previous := layoutPreviewFn
	var applied []string
	layoutPreviewFn = func(_, layout string) error { applied = append(applied, layout); return nil }
	t.Cleanup(func() { layoutPreviewFn = previous })
	m := NewModel(ModelConfig{})
	lvl := newLevel("window:layout", "layouts", []menu.Item{{ID: "tiled", Label: "tiled"}, {ID: "original", Label: "current layout"}}, nil)
	lvl.Cursor = lvl.IndexOf("tiled")
	m.stack = []*level{lvl}
	stale := m.ensurePreviewForLevel(lvl)
	cmd := m.handleEscapeKey()
	if cmd == nil {
		t.Fatal("missing quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected quit after restore, got %T", msg)
	}
	stale()
	if len(applied) != 1 || applied[0] != "original" {
		t.Fatalf("root escape failed to restore layout: %v", applied)
	}
}

func TestLayoutConfirmationOwnsFinalMutation(t *testing.T) {
	previous := layoutPreviewFn
	var applied []string
	layoutPreviewFn = func(_, layout string) error { applied = append(applied, layout); return nil }
	t.Cleanup(func() { layoutPreviewFn = previous })
	m := NewModel(ModelConfig{})
	node := &menu.Node{ID: "window:layout", Action: func(menu.Context, menu.Item) tea.Cmd {
		return func() tea.Msg { applied = append(applied, "confirmed"); return menu.ActionResult{} }
	}}
	lvl := newLevel("window:layout", "layouts", []menu.Item{{ID: "tiled", Label: "tiled"}, {ID: "original", Label: "current layout"}}, node)
	lvl.Cursor = lvl.IndexOf("tiled")
	m.stack = []*level{lvl}
	stale := m.ensurePreviewForLevel(lvl)
	m.handleEnterKey()()
	stale()
	if len(applied) != 1 || applied[0] != "confirmed" {
		t.Fatalf("old preview overwrote confirmation: %v", applied)
	}
}

func TestPluginEscapeCancelsQueuedSideEffect(t *testing.T) {
	m := NewModel(ModelConfig{})
	dir := t.TempDir()
	target := dir + "/owned-plugin"
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	m.startPluginProgress([]plugin.Plugin{{Name: "owned-plugin", Dir: target}}, dir, "update")
	queued := m.runPluginInstallStage(0, pluginInstallRemoving)
	m.handlePluginInstallKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	queued()
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("cancelled operation removed plugin: %v", err)
	}
}

func TestLayoutRunningPreviewFinishesBeforeRestore(t *testing.T) {
	previous := layoutPreviewFn
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var applied []string
	layoutPreviewFn = func(_, layout string) error {
		if layout == "tiled" {
			close(started)
			<-release
		}
		applied = append(applied, layout)
		return nil
	}
	t.Cleanup(func() { layoutPreviewFn = previous })
	m := NewModel(ModelConfig{})
	lvl := newLevel("window:layout", "layouts", []menu.Item{{ID: "tiled", Label: "tiled"}, {ID: "original", Label: "current layout"}}, nil)
	lvl.Cursor = lvl.IndexOf("tiled")
	m.stack = append(m.stack, lvl)
	preview := m.ensurePreviewForLevel(lvl)
	previewDone := make(chan struct{})
	go func() { preview(); close(previewDone) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("preview did not start")
	}
	restore := m.handleEscapeKey()
	restoreDone := make(chan struct{})
	go func() { restore(); close(restoreDone) }()
	select {
	case <-restoreDone:
		unblock()
		<-previewDone
		t.Fatal("restore ran concurrently with pending layout mutation")
	case <-time.After(20 * time.Millisecond):
	}
	unblock()
	<-previewDone
	<-restoreDone
	if len(applied) != 2 || applied[0] != "tiled" || applied[1] != "original" {
		t.Fatalf("unexpected mutation order: %v", applied)
	}
}

func TestModelCloseCancelsPluginOperation(t *testing.T) {
	m := NewModel(ModelConfig{})
	dir := t.TempDir()
	m.startPluginProgress([]plugin.Plugin{{Name: "plugin", Dir: dir + "/plugin"}}, dir, "update")
	operation := m.pluginInstallState
	closer, ok := any(m).(interface{ Close() })
	if !ok {
		t.Fatal("model has no shutdown hook to cancel background work")
	}
	closer.Close()
	if operation.ctx.Err() == nil {
		t.Fatal("model close left plugin operation active")
	}
}

func TestPluginOverviewIsScheduledAsCommand(t *testing.T) {
	m := NewModel(ModelConfig{})
	lvl := newLevel("plugins", "plugins", []menu.Item{{ID: "plugins:install", Label: "install"}}, nil)
	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatal("plugin overview loaded synchronously instead of returning a command")
	}
	if !m.preview[lvl.ID].loading {
		t.Fatal("plugin overview missing in-flight ownership")
	}
	if overlapping := m.ensurePreviewForLevel(lvl); overlapping != nil {
		t.Fatal("plugin overview scheduled overlapping read")
	}
}

func TestLayoutReentryPreservesPendingRollback(t *testing.T) {
	previous := layoutPreviewFn
	var applied []string
	layoutPreviewFn = func(_, layout string) error { applied = append(applied, layout); return nil }
	t.Cleanup(func() { layoutPreviewFn = previous })
	m := NewModel(ModelConfig{})
	first := newLevel("window:layout", "layouts", []menu.Item{{ID: "tiled"}, {ID: "original", Label: "current layout"}}, nil)
	first.Cursor = first.IndexOf("tiled")
	m.stack = append(m.stack, first)
	m.ensurePreviewForLevel(first)()
	rollback := m.handleEscapeKey()
	// The backend can still report the preview layout before rollback executes.
	second := newLevel("window:layout", "layouts", []menu.Item{{ID: "even-horizontal"}, {ID: "tiled", Label: "current layout"}}, nil)
	second.Cursor = second.IndexOf("even-horizontal")
	m.stack = append(m.stack, second)
	preview := m.ensurePreviewForLevel(second)
	preview() // must execute the required rollback even if its command was never scheduled.
	rollback()
	m.handleEscapeKey()()
	want := []string{"tiled", "original", "even-horizontal", "original"}
	if !slices.Equal(applied, want) {
		t.Fatalf("layout rollback order = %v, want %v", applied, want)
	}
}

func TestModelClosePreventsQueuedLayoutMutation(t *testing.T) {
	previous := layoutPreviewFn
	var applied []string
	layoutPreviewFn = func(_, layout string) error { applied = append(applied, layout); return nil }
	t.Cleanup(func() { layoutPreviewFn = previous })
	m := NewModel(ModelConfig{})
	lvl := newLevel("window:layout", "layouts", []menu.Item{{ID: "tiled"}, {ID: "original", Label: "current layout"}}, nil)
	m.stack = append(m.stack, lvl)
	queued := m.ensurePreviewForLevel(lvl)
	m.Close()
	before := len(applied)
	queued()
	if len(applied) != before {
		t.Fatalf("queued command mutated layout after close: %v", applied)
	}
}

func TestModelCloseDoesNotWaitForRunningLayout(t *testing.T) {
	previous := layoutPreviewFn
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var applied []string
	layoutPreviewFn = func(_, layout string) error {
		if layout == "tiled" {
			close(started)
			<-release
		}
		applied = append(applied, layout)
		return nil
	}
	t.Cleanup(func() { layoutPreviewFn = previous })
	m := NewModel(ModelConfig{})
	lvl := newLevel("window:layout", "layouts", []menu.Item{{ID: "tiled"}, {ID: "original", Label: "current layout"}}, nil)
	lvl.Cursor = lvl.IndexOf("tiled")
	m.stack = append(m.stack, lvl)
	running := m.ensurePreviewForLevel(lvl)
	done := make(chan struct{})
	go func() { running(); close(done) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("layout did not start")
	}
	rollback := m.handleEscapeKey()
	closed := make(chan struct{})
	go func() { m.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		unblock()
		<-done
		<-closed
		t.Fatal("close waited for running tmux operation")
	}
	unblock()
	<-done
	rollback()
	if !slices.Equal(applied, []string{"tiled"}) {
		t.Fatalf("queued rollback mutated after terminal shutdown: %v", applied)
	}
}
