package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/extract"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// ctrlF constructs the ctrl+f key press message. Verified against the
// bubbletea v2 vendor (charmbracelet/ultraviolet key.go Keystroke()): with an
// empty Text field, String() falls back to Keystroke(), which prefixes
// "ctrl+" for ModCtrl and appends the rune for Code 'f', yielding "ctrl+f".
func ctrlF() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl}
}

func TestExtractCycleAdvancesCategoryAndReloads(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "open https://example.com file internal/x.go", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	if got := h.Model().extractCategory; got != extract.Word {
		t.Fatalf("initial category = %v, want word", got)
	}
	current := h.Model().currentLevel()
	if current == nil || current.ID != extractLevelID {
		t.Fatalf("expected extract level to be current, got %+v", current)
	}

	h.Send(ctrlF())

	if got := h.Model().extractCategory; got != extract.Path {
		t.Fatalf("after ctrl+f category = %v, want path", got)
	}

	current = h.Model().currentLevel()
	found := false
	for _, item := range current.Items {
		if item.ID == "internal/x.go" {
			found = true
			break
		}
	}
	if !found {
		ids := make([]string, 0, len(current.Items))
		for _, item := range current.Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("path items missing internal/x.go: %v", ids)
	}
}

// TestExtractReentryResetsToWord reproduces the reported bug: cycling to a
// non-default category, escaping back to root, then re-entering extract must
// reset both the header (extractCategory) and the actual items back to Word
// — not just the header, with items staying on the stale category until
// another ctrl-f.
func TestExtractReentryResetsToWord(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello https://example.com world", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "", SocketPath: "test.sock"})
	h := NewHarness(m)

	// Root level; "extract" is the first RootItems entry. The level's
	// initial cursor defaults to the last item (see Level.applyFilter), so
	// position the cursor on "extract" explicitly before pressing Enter.
	root := h.Model().currentLevel()
	if root == nil || root.ID != "root" {
		t.Fatalf("expected root level, got %+v", root)
	}
	idx := root.IndexOf("extract")
	if idx != 0 {
		t.Fatalf("expected extract item at index 0, got %d", idx)
	}
	root.Cursor = idx

	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	current := h.Model().currentLevel()
	if current == nil || current.ID != extractLevelID {
		t.Fatalf("expected extract level to be current after Enter, got %+v", current)
	}
	if got := h.Model().extractCategory; got != extract.Word {
		t.Fatalf("initial category = %v, want word", got)
	}

	// Cycle to a non-default category.
	h.Send(ctrlF())
	if got := h.Model().extractCategory; got != extract.Path {
		t.Fatalf("after ctrl+f category = %v, want path", got)
	}

	// Escape back to root.
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if root := h.Model().currentLevel(); root == nil || root.ID != "root" {
		t.Fatalf("expected root level after escape, got %+v", root)
	}

	// Re-enter extract.
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := h.Model().extractCategory; got != extract.Word {
		t.Fatalf("re-entry category = %v, want word (header regression)", got)
	}
	current = h.Model().currentLevel()
	if current == nil || current.ID != extractLevelID {
		t.Fatalf("expected extract level to be current after re-entry, got %+v", current)
	}
	foundWord, foundURLOnly := false, true
	for _, item := range current.Items {
		if item.ID == "hello" {
			foundWord = true
		}
		if item.ID != "https://example.com" {
			foundURLOnly = false
		}
	}
	if !foundWord {
		ids := make([]string, 0, len(current.Items))
		for _, item := range current.Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("re-entry items missing word token %q: %v (items desynced from reset header)", "hello", ids)
	}
	if foundURLOnly {
		t.Fatalf("re-entry items still URL-only, category reset did not propagate to loader")
	}
}

// TestExtractCycleKeepsSingleLevel verifies ctrl-f updates the current
// extract level in place rather than pushing a new stack level.
func TestExtractCycleKeepsSingleLevel(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello https://example.com world", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	before := len(h.Model().stack)
	h.Send(ctrlF())
	after := len(h.Model().stack)
	if before != after {
		t.Fatalf("stack depth changed on ctrl-f: before=%d after=%d, want unchanged (in-place reload)", before, after)
	}
}

// TestExtractStaleReloadIgnored verifies that an extractReloadMsg carrying an
// older seq than the model's current extractSeq is dropped, so an
// out-of-order async reply from an earlier ctrl-f cannot clobber the items
// belonging to a later one (see internal/ui/preview.go's seq guard for the
// established pattern).
func TestExtractStaleReloadIgnored(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello world", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	current := h.Model().currentLevel()
	wantItems := make([]menu.Item, len(current.Items))
	copy(wantItems, current.Items)

	h.Model().extractSeq = 5

	bogus := []menu.Item{{ID: "bogus", Label: "bogus"}}
	h.Send(extractReloadMsg{items: bogus, seq: 3})

	current = h.Model().currentLevel()
	if len(current.Items) != len(wantItems) {
		t.Fatalf("stale reload changed item count: got %d, want %d (bogus items applied)", len(current.Items), len(wantItems))
	}
	for _, item := range current.Items {
		if item.ID == "bogus" {
			t.Fatalf("stale reload (seq 3 < current 5) was applied: found bogus item")
		}
	}
}

// TestExtractEntryInvalidatesStaleReload proves that (re)entering the extract
// level bumps m.extractSeq, so a ctrl-f reload dispatched during an earlier
// visit cannot land after the user has navigated away and back in. Without
// the entry-time seq bump, nothing invalidates the earlier request's
// captured seq, so a stale extractReloadMsg would still satisfy
// reload.seq == m.extractSeq and clobber the freshly-reset items.
func TestExtractEntryInvalidatesStaleReload(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello https://example.com world", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "", SocketPath: "test.sock"})
	h := NewHarness(m)

	root := h.Model().currentLevel()
	if root == nil || root.ID != "root" {
		t.Fatalf("expected root level, got %+v", root)
	}
	idx := root.IndexOf("extract")
	if idx != 0 {
		t.Fatalf("expected extract item at index 0, got %d", idx)
	}
	root.Cursor = idx

	// Enter extract, then cycle once via ctrl-f. This bumps extractSeq and
	// dispatches (and, in the harness, resolves) a reload stamped with that
	// seq.
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	current := h.Model().currentLevel()
	if current == nil || current.ID != extractLevelID {
		t.Fatalf("expected extract level to be current after Enter, got %+v", current)
	}
	h.Send(ctrlF())
	staleSeq := h.Model().extractSeq

	// Escape back to root, then re-enter extract.
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if root := h.Model().currentLevel(); root == nil || root.ID != "root" {
		t.Fatalf("expected root level after escape, got %+v", root)
	}
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = h.Model().currentLevel()
	if current == nil || current.ID != extractLevelID {
		t.Fatalf("expected extract level to be current after re-entry, got %+v", current)
	}

	if got := h.Model().extractSeq; got <= staleSeq {
		t.Fatalf("re-entry did not bump extractSeq: got %d, want > %d", got, staleSeq)
	}

	// Record the freshly-loaded items, then feed in a reload stamped with the
	// stale (prior-visit) seq. It must be dropped rather than overwriting
	// the current items.
	wantItems := make([]menu.Item, len(current.Items))
	copy(wantItems, current.Items)

	bogus := []menu.Item{{ID: "bogus", Label: "bogus"}}
	h.Send(extractReloadMsg{items: bogus, seq: staleSeq})

	current = h.Model().currentLevel()
	if len(current.Items) != len(wantItems) {
		t.Fatalf("stale prior-visit reload changed item count: got %d, want %d (bogus items applied)", len(current.Items), len(wantItems))
	}
	for _, item := range current.Items {
		if item.ID == "bogus" {
			t.Fatalf("stale prior-visit reload (seq %d) was applied despite entry-time seq bump", staleSeq)
		}
	}
}

// TestExtractEnterInsertsSelectedToken verifies that pressing enter on the
// extract level inserts the token under the cursor into the origin pane
// (via the injectable extractInsertFn) and quits.
func TestExtractEnterInsertsSelectedToken(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", "%9")
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "run internal/target.go now", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	// Cycle from the default word category to path so the cursor can land on
	// a path token.
	h.Send(ctrlF())
	if got := h.Model().extractCategory; got != extract.Path {
		t.Fatalf("category after ctrl+f = %v, want path", got)
	}

	current := h.Model().currentLevel()
	idx := current.IndexOf("internal/target.go")
	if idx < 0 {
		ids := make([]string, 0, len(current.Items))
		for _, item := range current.Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("path items missing internal/target.go: %v", ids)
	}
	current.Cursor = idx

	origInsert := extractInsertFn
	var inserted struct{ target, text string }
	extractInsertFn = func(sock, target, text string) error {
		inserted.target, inserted.text = target, text
		return nil
	}
	defer func() { extractInsertFn = origInsert }()

	_, cmd := h.Model().Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected a command from enter")
	}
	msg := cmd()
	done, ok := msg.(extractDoneMsg)
	if !ok {
		t.Fatalf("expected extractDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if inserted.text != "internal/target.go" {
		t.Fatalf("inserted text = %q, want %q", inserted.text, "internal/target.go")
	}
	if inserted.target != "%9" {
		t.Fatalf("inserted target = %q, want %%9", inserted.target)
	}

	_, cmd2 := h.Model().Update(msg)
	if cmd2 == nil {
		t.Fatalf("expected a quit command after successful insert")
	}
	qmsg := cmd2()
	if _, ok := qmsg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", qmsg)
	}
}

// TestExtractCtrlYCopiesSelectedToken verifies that ctrl-y on the extract
// level copies the token under the cursor (via extractCopyFn) and quits.
func TestExtractCtrlYCopiesSelectedToken(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	current := h.Model().currentLevel()
	idx := current.IndexOf("please")
	if idx < 0 {
		ids := make([]string, 0, len(current.Items))
		for _, item := range current.Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("word items missing please: %v", ids)
	}
	current.Cursor = idx

	origCopy := extractCopyFn
	var copied string
	extractCopyFn = func(sock, text string) error {
		copied = text
		return nil
	}
	defer func() { extractCopyFn = origCopy }()

	ctrlY := tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl}
	if got := ctrlY.String(); got != "ctrl+y" {
		t.Fatalf("ctrl+y key string = %q, want ctrl+y", got)
	}

	_, cmd := h.Model().Update(ctrlY)
	if cmd == nil {
		t.Fatalf("expected a command from ctrl+y")
	}
	msg := cmd()
	done, ok := msg.(extractDoneMsg)
	if !ok {
		t.Fatalf("expected extractDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if copied != "please" {
		t.Fatalf("copied = %q, want %q", copied, "please")
	}

	_, cmd2 := h.Model().Update(msg)
	if cmd2 == nil {
		t.Fatalf("expected a quit command after successful copy")
	}
	qmsg := cmd2()
	if _, ok := qmsg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", qmsg)
	}
}

// TestExtractMultiSelectJoinsWithSpace verifies that marking multiple word
// tokens with tab, then copying with ctrl-y, joins the marked tokens with a
// space (word/path/url/quote categories use space; All/Line use newline).
func TestExtractMultiSelectJoinsWithSpace(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "alpha bravo charlie", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	current := h.Model().currentLevel()
	if !current.MultiSelect {
		t.Fatalf("expected extract level to be multi-select")
	}

	idxAlpha := current.IndexOf("alpha")
	idxBravo := current.IndexOf("bravo")
	if idxAlpha < 0 || idxBravo < 0 {
		ids := make([]string, 0, len(current.Items))
		for _, item := range current.Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("word items missing alpha/bravo: %v", ids)
	}

	current.Cursor = idxAlpha
	h.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	current = h.Model().currentLevel()
	current.Cursor = idxBravo
	h.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	origCopy := extractCopyFn
	var copied string
	extractCopyFn = func(sock, text string) error {
		copied = text
		return nil
	}
	defer func() { extractCopyFn = origCopy }()

	_, cmd := h.Model().Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatalf("expected a command from ctrl+y")
	}
	msg := cmd()
	done, ok := msg.(extractDoneMsg)
	if !ok {
		t.Fatalf("expected extractDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if copied != "alpha bravo" && copied != "bravo alpha" {
		t.Fatalf("copied = %q, want space-joined alpha/bravo", copied)
	}
}

func TestExtractCyclePreservesFilterQuery(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "alpha https://alpha.dev/x beta internal/beta.go", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	current := h.Model().currentLevel()
	current.SetFilter("be", 2)

	h.Send(ctrlF())

	if got := h.Model().currentLevel().Filter; got != "be" {
		t.Fatalf("filter after cycle = %q, want %q", got, "be")
	}
}
