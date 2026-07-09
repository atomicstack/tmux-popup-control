package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/extract"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/charmbracelet/x/ansi"
)

// ctrlF constructs the ctrl+f key press message. Verified against the
// bubbletea v2 vendor (charmbracelet/ultraviolet key.go Keystroke()): with an
// empty Text field, String() falls back to Keystroke(), which prefixes
// "ctrl+" for ModCtrl and appends the rune for Code 'f', yielding "ctrl+f".
func ctrlF() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl}
}

// ctrlG constructs the ctrl+g key press message, the hotkey for the area
// selector popup (see ctrlF for the verification this construction relies on).
func ctrlG() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}
}

// extractMark toggles multi-select on the current extract row (shift+tab).
func extractMark(h *Harness) {
	h.Send(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
}

// extractSelectCategory opens the mode popup (first ctrl-f) and cycles to
// target (subsequent ctrl-f presses), then confirms with enter — leaving the
// extract level on that category with the popup closed.
func extractSelectCategory(t *testing.T, h *Harness, target extract.Category) {
	t.Helper()
	h.Send(ctrlF()) // open popup at current category (no change)
	for guard := 0; h.Model().extractCategory != target; guard++ {
		if guard > 20 {
			t.Fatalf("could not reach category %v via ctrl-f", target)
		}
		h.Send(ctrlF()) // advance to next mode
	}
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter}) // confirm + close popup
}

// extractSelectArea opens the area popup (first ctrl-g) and cycles to target
// (subsequent ctrl-g presses), then confirms with enter — leaving the extract
// level on that area with the popup closed.
func extractSelectArea(t *testing.T, h *Harness, target extract.GrabArea) {
	t.Helper()
	h.Send(ctrlG()) // open popup at current area (no change)
	for guard := 0; h.Model().extractGrabArea != target; guard++ {
		if guard > 20 {
			t.Fatalf("could not reach area %v via ctrl-g", target)
		}
		h.Send(ctrlG()) // advance to next area
	}
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter}) // confirm + close popup
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

	// First ctrl-f opens the mode popup without changing the category.
	h.Send(ctrlF())
	if got := h.Model().extractCategory; got != extract.Word {
		t.Fatalf("opening the mode popup should not change the category, got %v", got)
	}
	if !h.Model().extractModePopupVisible() {
		t.Fatalf("first ctrl-f should open the mode popup")
	}
	// Subsequent ctrl-f advances to the next mode (path) and re-extracts live.
	h.Send(ctrlF())

	if got := h.Model().extractCategory; got != extract.Path {
		t.Fatalf("after second ctrl+f category = %v, want path", got)
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

	// Cycle to a non-default category via the mode popup.
	extractSelectCategory(t, h, extract.Path)
	if got := h.Model().extractCategory; got != extract.Path {
		t.Fatalf("after selecting path category = %v, want path", got)
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
	extractSelectCategory(t, h, extract.Path)
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

	// Select the path category (via the mode popup) so the cursor can land on
	// a path token.
	extractSelectCategory(t, h, extract.Path)
	if got := h.Model().extractCategory; got != extract.Path {
		t.Fatalf("category after selecting path = %v, want path", got)
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
	extractMark(h)
	current = h.Model().currentLevel()
	current.Cursor = idxBravo
	extractMark(h)

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

// TestExtractDirectInvocationEscapeQuits verifies that pressing Escape when the
// extract level is directly invoked (via RootMenu: "extract") quits the app
// rather than popping to a root menu. Per the direct-invocation convention,
// forms/submenus invoked via hotkey must quit on escape.
func TestExtractDirectInvocationEscapeQuits(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	// Stack has only the extract level (direct invocation).
	if len(h.Model().stack) != 1 {
		t.Fatalf("direct-invocation stack depth = %d, want 1", len(h.Model().stack))
	}
	if got := h.Model().currentLevel().ID; got != extractLevelID {
		t.Fatalf("current level ID = %q, want extract", got)
	}

	// Escape should return a quit command.
	_, cmd := h.Model().Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatalf("escape from directly-invoked extract should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestExtractFromRootMenuEscapeReturnsToRoot verifies that pressing Escape when
// the extract level is entered from the root menu pops back to root, not quit.
// This demonstrates the distinction between direct invocation (quit on escape)
// and navigating into a submenu (pop on escape).
func TestExtractFromRootMenuEscapeReturnsToRoot(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "", SocketPath: "test.sock"})
	h := NewHarness(m)

	// Verify starting state: at root level.
	if got := h.Model().currentLevel().ID; got != "root" {
		t.Fatalf("initial level ID = %q, want root", got)
	}

	// Navigate to extract level by finding "extract" item (first in root) and pressing Enter.
	root := h.Model().currentLevel()
	idx := root.IndexOf("extract")
	if idx != 0 {
		t.Fatalf("expected extract item at index 0, got %d", idx)
	}
	root.Cursor = idx
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Verify we're now in extract level.
	current := h.Model().currentLevel()
	if got := current.ID; got != extractLevelID {
		t.Fatalf("after enter, level ID = %q, want extract", got)
	}

	// Escape should pop back to root, not quit.
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	if got := h.Model().currentLevel().ID; got != "root" {
		t.Fatalf("after escape, level ID = %q, want root", got)
	}
}

// TestExtractMultiSelectAllJoinsWithNewline verifies that selecting multiple
// tokens in the All category and copying them joins them with a newline rather
// than space. The All category treats each token as a whole line, consistent
// with Line-category semantics. This closes a Task-7 coverage gap for newline-
// join behavior.
func TestExtractMultiSelectAllJoinsWithNewline(t *testing.T) {
	// Use a capture string with multiple URLs and paths, which the All category
	// will extract. The All category draws from Path, URL, Quote, SQuote.
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "https://one.example.com /path/to/file https://two.example.com", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	// Verify initial category is Word.
	if got := h.Model().extractCategory; got != extract.Word {
		t.Fatalf("initial category = %v, want word", got)
	}

	// Select the All category via the mode popup.
	extractSelectCategory(t, h, extract.All)

	if got := h.Model().extractCategory; got != extract.All {
		t.Fatalf("after cycling, category = %v, want all", got)
	}

	current := h.Model().currentLevel()
	if !current.MultiSelect {
		t.Fatalf("expected extract level to be multi-select")
	}

	// Find two extractable items: a URL and a path.
	urlIdx := current.IndexOf("https://one.example.com")
	pathIdx := current.IndexOf("/path/to/file")

	if urlIdx < 0 || pathIdx < 0 {
		ids := make([]string, 0, len(current.Items))
		for _, item := range current.Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("All category missing URL/path items: %v", ids)
	}

	// Select two items using shift+tab.
	current.Cursor = urlIdx
	extractMark(h)

	current = h.Model().currentLevel()
	current.Cursor = pathIdx
	extractMark(h)

	// Stub extractCopyFn to capture the text.
	origCopy := extractCopyFn
	var copied string
	extractCopyFn = func(sock, text string) error {
		copied = text
		return nil
	}
	defer func() { extractCopyFn = origCopy }()

	// Press ctrl+y to copy the selected items.
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

	// Verify copied text contains newline (order may vary due to marked order).
	if !strings.Contains(copied, "\n") {
		t.Fatalf("copied text %q does not contain newline (All category must use newline join)", copied)
	}

	// Verify both items are present.
	if !strings.Contains(copied, "https://one.example.com") || !strings.Contains(copied, "/path/to/file") {
		t.Fatalf("copied text missing one or both items: %q", copied)
	}
}

// TestExtractModeMenuRendersBelowListAboveInput asserts the category (mode)
// header is rendered BELOW the token list and ABOVE the fuzzy filter input,
// per the requested layout.
func TestExtractModeMenuRendersBelowListAboveInput(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := h.View()
	tokenIdx := strings.Index(view, "please")          // an extracted token in the list
	headerIdx := strings.Index(view, "mode:")          // the mode line (mode: <current> <^f>)
	promptIdx := strings.Index(view, "type to search") // the fuzzy input placeholder
	if tokenIdx < 0 || headerIdx < 0 || promptIdx < 0 {
		t.Fatalf("missing markers: token=%d header=%d prompt=%d\nview:\n%s", tokenIdx, headerIdx, promptIdx, view)
	}
	if tokenIdx >= headerIdx {
		t.Fatalf("expected token list ABOVE mode menu; token=%d header=%d", tokenIdx, headerIdx)
	}
	if headerIdx >= promptIdx {
		t.Fatalf("expected mode menu ABOVE fuzzy input; header=%d prompt=%d", headerIdx, promptIdx)
	}
}

// TestExtractModeLabelsAboveSeparatorAboveInput asserts the extract mode
// labels sit immediately above the separator, which sits directly above the
// fuzzy input on the last row.
func TestExtractModeLabelsAboveSeparatorAboveInput(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	lines := strings.Split(h.View(), "\n")
	last := len(lines) - 1
	if !strings.Contains(lines[last], "type to search") {
		t.Fatalf("expected fuzzy input on the last row, got %q", lines[last])
	}
	if !strings.Contains(lines[last-1], "─") {
		t.Fatalf("expected separator directly above input, got %q", lines[last-1])
	}
	if !strings.Contains(lines[last-2], "mode:") {
		t.Fatalf("expected mode line immediately above the separator, got %q", lines[last-2])
	}
}

// TestExtractMultiSelectUsesVerticalBarNotCheckbox asserts the extract list
// uses an extrakto/fzf-style coloured vertical bar for selected rows instead
// of the ■/□ checkboxes used elsewhere.
func TestExtractMultiSelectUsesVerticalBarNotCheckbox(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	before := ansi.Strip(h.View())
	if strings.ContainsAny(before, "■□") {
		t.Fatalf("extract list should not render checkboxes, got:\n%s", before)
	}
	// Mark the row under the cursor.
	extractMark(h)
	after := ansi.Strip(h.View())
	// ┃ (U+2503 box drawings heavy vertical): the fzf-style selection bar.
	if !strings.Contains(after, "┃") {
		t.Fatalf("expected a heavy vertical bar marker for the selected extract row, got:\n%s", after)
	}
	if strings.ContainsAny(after, "■□") {
		t.Fatalf("extract selection must not use checkboxes, got:\n%s", after)
	}
}

// TestNonExtractMultiSelectStillUsesCheckbox guards that the checkbox marker is
// unchanged for other multi-select menus (e.g. pane:kill).
func TestNonExtractMultiSelectStillUsesCheckbox(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	lvl := newLevel("pane:kill", "pane:kill", []menu.Item{{ID: "%1", Label: "pane one"}}, nil)
	lvl.MultiSelect = true
	line := m.buildItemLine(lvl.Items[0], 0, lvl, 80)
	stripped := ansi.Strip(line.text)
	if !strings.Contains(stripped, "□") {
		t.Fatalf("non-extract multiselect should still use a checkbox, got %q", stripped)
	}
}

// --- mode selector popup ---

func TestExtractModeLineRendersCurrentMode(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := ansi.Strip(h.View())
	if !strings.Contains(view, "mode: word <^f>") {
		t.Fatalf("expected mode line 'mode: word <^f>', got:\n%s", view)
	}
	if strings.Contains(view, "s-quote  line  all") {
		t.Fatalf("mode line should not list every category anymore")
	}
}

func TestExtractModePopupEscRevertsToPrePopupMode(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello https://example.com world", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	h.Send(ctrlF()) // open at word
	h.Send(ctrlF()) // advance to path
	if h.Model().extractCategory != extract.Path {
		t.Fatalf("want path after advance, got %v", h.Model().extractCategory)
	}
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape}) // revert
	if h.Model().extractCategory != extract.Word {
		t.Fatalf("esc should revert to word, got %v", h.Model().extractCategory)
	}
	if h.Model().extractModePopupVisible() {
		t.Fatalf("esc should close the popup")
	}
	if h.Model().currentLevel().ID != extractLevelID {
		t.Fatalf("esc in the popup must not leave the extract level")
	}
}

func TestExtractModePopupEnterKeepsMode(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello https://example.com world", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	h.Send(ctrlF())
	h.Send(ctrlF()) // advance to path
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if h.Model().extractModePopupVisible() {
		t.Fatalf("enter should close the popup")
	}
	if h.Model().extractCategory != extract.Path {
		t.Fatalf("enter should keep the selected mode (path), got %v", h.Model().extractCategory)
	}
	if h.Model().currentLevel().ID != extractLevelID {
		t.Fatalf("enter in the popup must not leave the extract level")
	}
}

func TestExtractModePopupUpWrapsToPreviousMode(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello world", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	h.Send(ctrlF())                          // open at word (index 0)
	h.Send(tea.KeyPressMsg{Code: tea.KeyUp}) // up wraps to the last mode (all)
	if h.Model().extractCategory != extract.All {
		t.Fatalf("up from word should wrap to all, got %v", h.Model().extractCategory)
	}
}

func TestExtractModePopupTimeoutClosesAndStaleIgnored(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello world", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	h.Send(ctrlF()) // open, arms the inactivity timer
	seq := h.Model().extractModeSeq
	if !h.Model().extractModePopupVisible() {
		t.Fatal("popup should be open after ctrl-f")
	}
	// A stale timeout (superseded by later activity) must not close it.
	h.Send(extractModeTimeoutMsg{seq: seq - 1})
	if !h.Model().extractModePopupVisible() {
		t.Fatal("stale timeout should be ignored")
	}
	// The current timeout closes it, keeping the current mode.
	h.Send(extractModeTimeoutMsg{seq: seq})
	if h.Model().extractModePopupVisible() {
		t.Fatal("matching timeout should close the popup")
	}
}

func TestExtractTabCopiesShiftTabMarks(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	origCopy := extractCopyFn
	var copied string
	extractCopyFn = func(sock, text string) error { copied = text; return nil }
	defer func() { extractCopyFn = origCopy }()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
	cur := h.Model().currentLevel()
	cur.Cursor = cur.IndexOf("please")

	// tab copies the cursor token and does NOT toggle selection.
	_, cmd := h.Model().Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd == nil {
		t.Fatal("tab should return a copy command")
	}
	msg := cmd()
	if done, ok := msg.(extractDoneMsg); !ok || done.err != nil {
		t.Fatalf("tab should produce a successful copy, got %T", msg)
	}
	if copied != "please" {
		t.Fatalf("tab copied %q, want please", copied)
	}
	if n := len(h.Model().currentLevel().SelectedItems()); n != 0 {
		t.Fatalf("tab must not toggle selection, got %d marked", n)
	}

	// shift+tab marks the row.
	extractMark(h)
	if n := len(h.Model().currentLevel().SelectedItems()); n != 1 {
		t.Fatalf("shift+tab should mark the row, got %d marked", n)
	}
}

func TestNonExtractTabStillToggles(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	lvl := newLevel("pane:kill", "pane:kill", []menu.Item{{ID: "%1", Label: "p1"}, {ID: "%2", Label: "p2"}}, nil)
	lvl.MultiSelect = true
	m.stack = []*level{lvl}
	h := NewHarness(m)
	h.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	if n := len(h.Model().currentLevel().SelectedItems()); n != 1 {
		t.Fatalf("tab should still mark on a non-extract multi-select level, got %d", n)
	}
}

// TestExtractCopyAlsoCopiesToSystemClipboard verifies that a successful copy
// writes the selected token to both the tmux paste buffer (extractCopyFn) and
// the system clipboard (extractClipboardFn), then quits.
func TestExtractCopyAlsoCopiesToSystemClipboard(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()

	origCopy := extractCopyFn
	var bufferCopied string
	extractCopyFn = func(sock, text string) error {
		bufferCopied = text
		return nil
	}
	defer func() { extractCopyFn = origCopy }()

	origClipboard := extractClipboardFn
	var clipboardCopied string
	extractClipboardFn = func(text string) error {
		clipboardCopied = text
		return nil
	}
	defer func() { extractClipboardFn = origClipboard }()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	current := h.Model().currentLevel()
	idx := current.IndexOf("please")
	if idx < 0 {
		t.Fatalf("word items missing please")
	}
	current.Cursor = idx

	_, cmd := h.Model().Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd == nil {
		t.Fatalf("expected a command from tab")
	}
	msg := cmd()
	done, ok := msg.(extractDoneMsg)
	if !ok {
		t.Fatalf("expected extractDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if bufferCopied != "please" {
		t.Fatalf("buffer copied = %q, want please", bufferCopied)
	}
	if clipboardCopied != "please" {
		t.Fatalf("clipboard copied = %q, want please", clipboardCopied)
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

// TestExtractClipboardErrorStillQuits verifies that a system-clipboard
// failure is swallowed: the tmux buffer is the source of truth, so the copy
// still reports success and quits.
func TestExtractClipboardErrorStillQuits(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()

	origCopy := extractCopyFn
	extractCopyFn = func(sock, text string) error { return nil }
	defer func() { extractCopyFn = origCopy }()

	origClipboard := extractClipboardFn
	extractClipboardFn = func(text string) error { return errors.New("no clipboard tool") }
	defer func() { extractClipboardFn = origClipboard }()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	current := h.Model().currentLevel()
	idx := current.IndexOf("please")
	if idx < 0 {
		t.Fatalf("word items missing please")
	}
	current.Cursor = idx

	_, cmd := h.Model().Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd == nil {
		t.Fatalf("expected a command from tab")
	}
	msg := cmd()
	done, ok := msg.(extractDoneMsg)
	if !ok {
		t.Fatalf("expected extractDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("clipboard failure must not surface as an error, got %v", done.err)
	}

	_, cmd2 := h.Model().Update(msg)
	if cmd2 == nil {
		t.Fatalf("expected a quit command despite the clipboard failure")
	}
	qmsg := cmd2()
	if _, ok := qmsg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", qmsg)
	}
	if h.Model().errMsg != "" {
		t.Fatalf("errMsg = %q, want empty (clipboard errors must not surface)", h.Model().errMsg)
	}
}

// TestExtractBufferErrorDoesNotQuit verifies that a tmux buffer failure keeps
// today's behaviour: no quit, m.errMsg set, and the system clipboard is never
// attempted.
func TestExtractBufferErrorDoesNotQuit(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()

	origCopy := extractCopyFn
	extractCopyFn = func(sock, text string) error { return errors.New("set-buffer failed") }
	defer func() { extractCopyFn = origCopy }()

	origClipboard := extractClipboardFn
	clipboardCalled := false
	extractClipboardFn = func(text string) error {
		clipboardCalled = true
		return nil
	}
	defer func() { extractClipboardFn = origClipboard }()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	current := h.Model().currentLevel()
	idx := current.IndexOf("please")
	if idx < 0 {
		t.Fatalf("word items missing please")
	}
	current.Cursor = idx

	_, cmd := h.Model().Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd == nil {
		t.Fatalf("expected a command from tab")
	}
	msg := cmd()
	done, ok := msg.(extractDoneMsg)
	if !ok {
		t.Fatalf("expected extractDoneMsg, got %T", msg)
	}
	if done.err == nil {
		t.Fatalf("expected a buffer error")
	}
	if clipboardCalled {
		t.Fatalf("system clipboard must not be attempted when the tmux buffer write fails")
	}

	if _, cmd2 := h.Model().Update(msg); cmd2 != nil {
		t.Fatalf("expected no command (no quit) after a failed copy")
	}
	if h.Model().errMsg == "" {
		t.Fatalf("expected errMsg to be set after a failed copy")
	}
}

// --- area selector popup ---

// TestExtractAreaCtrlGOpensPopupAtCurrentArea verifies the first ctrl-g opens
// the area popup without changing the active area, mirroring the mode
// popup's opening behaviour.
func TestExtractAreaCtrlGOpensPopupAtCurrentArea(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)

	if got := h.Model().extractGrabArea; got != extract.Viewport {
		t.Fatalf("initial area = %v, want viewport", got)
	}
	if h.Model().extractAreaPopupVisible() {
		t.Fatalf("area popup should not be open before ctrl-g")
	}
	h.Send(ctrlG())
	if got := h.Model().extractGrabArea; got != extract.Viewport {
		t.Fatalf("opening the area popup should not change the area, got %v", got)
	}
	if !h.Model().extractAreaPopupVisible() {
		t.Fatalf("first ctrl-g should open the area popup")
	}
}

// TestExtractAreaCtrlGAdvancesWithWrap verifies repeated ctrl-g presses cycle
// through the grab-area order and wrap from the last back to the first.
func TestExtractAreaCtrlGAdvancesWithWrap(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)

	h.Send(ctrlG()) // open at viewport
	h.Send(ctrlG()) // advance to pane-history
	if got := h.Model().extractGrabArea; got != extract.PaneHistory {
		t.Fatalf("after second ctrl-g area = %v, want pane-history", got)
	}
	h.Send(ctrlG()) // advance to window
	if got := h.Model().extractGrabArea; got != extract.Window {
		t.Fatalf("after third ctrl-g area = %v, want window", got)
	}
	h.Send(ctrlG()) // advance to window-history
	if got := h.Model().extractGrabArea; got != extract.WindowHistory {
		t.Fatalf("after fourth ctrl-g area = %v, want window-history", got)
	}
	h.Send(ctrlG()) // wraps back to viewport
	if got := h.Model().extractGrabArea; got != extract.Viewport {
		t.Fatalf("ctrl-g should wrap from window-history back to viewport, got %v", got)
	}
}

// TestExtractAreaPopupUpWrapsToPreviousArea verifies up from the first area
// (viewport) wraps to the last (window-history), mirroring the mode popup.
func TestExtractAreaPopupUpWrapsToPreviousArea(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello world", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)

	h.Send(ctrlG())                          // open at viewport (index 0)
	h.Send(tea.KeyPressMsg{Code: tea.KeyUp}) // up wraps to the last area (window-history)
	if h.Model().extractGrabArea != extract.WindowHistory {
		t.Fatalf("up from viewport should wrap to window-history, got %v", h.Model().extractGrabArea)
	}
}

// TestExtractAreaPopupEnterKeepsArea verifies enter confirms the highlighted
// area and closes the popup without leaving the extract level.
func TestExtractAreaPopupEnterKeepsArea(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello world", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)

	h.Send(ctrlG())
	h.Send(ctrlG()) // advance to pane-history
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if h.Model().extractAreaPopupVisible() {
		t.Fatalf("enter should close the area popup")
	}
	if h.Model().extractGrabArea != extract.PaneHistory {
		t.Fatalf("enter should keep the selected area (pane-history), got %v", h.Model().extractGrabArea)
	}
	if h.Model().currentLevel().ID != extractLevelID {
		t.Fatalf("enter in the area popup must not leave the extract level")
	}
}

// TestExtractAreaPopupEscRevertsToPrePopupArea verifies esc reverts to the
// area active before the popup opened and closes the popup, mirroring the
// mode popup's esc-revert behaviour.
func TestExtractAreaPopupEscRevertsToPrePopupArea(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello world", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)

	h.Send(ctrlG()) // open at viewport
	h.Send(ctrlG()) // advance to pane-history
	if h.Model().extractGrabArea != extract.PaneHistory {
		t.Fatalf("want pane-history after advance, got %v", h.Model().extractGrabArea)
	}
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape}) // revert
	if h.Model().extractGrabArea != extract.Viewport {
		t.Fatalf("esc should revert to viewport, got %v", h.Model().extractGrabArea)
	}
	if h.Model().extractAreaPopupVisible() {
		t.Fatalf("esc should close the area popup")
	}
	if h.Model().currentLevel().ID != extractLevelID {
		t.Fatalf("esc in the area popup must not leave the extract level")
	}
}

// TestExtractAreaPopupTimeoutClosesAndStaleIgnored mirrors
// TestExtractModePopupTimeoutClosesAndStaleIgnored for the area popup: both
// selectors share the same inactivity timer/seq (extractModeSeq /
// extractModeTimeoutMsg), so a stale timeout must be ignored and a matching
// one must close whichever popup is open.
func TestExtractAreaPopupTimeoutClosesAndStaleIgnored(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "hello world", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	h.Send(ctrlG()) // open, arms the shared inactivity timer
	seq := h.Model().extractModeSeq
	if !h.Model().extractAreaPopupVisible() {
		t.Fatal("area popup should be open after ctrl-g")
	}
	// A stale timeout (superseded by later activity) must not close it.
	h.Send(extractModeTimeoutMsg{seq: seq - 1})
	if !h.Model().extractAreaPopupVisible() {
		t.Fatal("stale timeout should be ignored")
	}
	// The current timeout closes it, keeping the current area.
	h.Send(extractModeTimeoutMsg{seq: seq})
	if h.Model().extractAreaPopupVisible() {
		t.Fatal("matching timeout should close the area popup")
	}
}

// TestExtractAreaChangeUpdatesMenuContextAndReloadsItems verifies that
// selecting a new area is wired into menuContext().ExtractGrabArea (so the
// loader's captureForArea picks it up) and that the change triggers a live
// re-extract (extractSeq bump), not just a header update.
func TestExtractAreaChangeUpdatesMenuContextAndReloadsItems(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	if got := h.Model().menuContext().ExtractGrabArea; got != extract.Viewport {
		t.Fatalf("initial menuContext ExtractGrabArea = %v, want viewport", got)
	}
	seqBefore := h.Model().extractSeq

	extractSelectArea(t, h, extract.PaneHistory)

	if got := h.Model().menuContext().ExtractGrabArea; got != extract.PaneHistory {
		t.Fatalf("menuContext ExtractGrabArea after selecting pane-history = %v, want pane-history", got)
	}
	if got := h.Model().extractSeq; got <= seqBefore {
		t.Fatalf("selecting a new area did not trigger a reload: extractSeq = %d, want > %d", got, seqBefore)
	}
}

// TestExtractCtrlGInertWhileModePopupOpen and
// TestExtractCtrlFInertWhileAreaPopupOpen guard the one-popup-at-a-time rule:
// only the currently open selector's own hotkey (or down/up/enter/esc) does
// anything; the other selector's hotkey is a handled no-op. This test must
// go red if ctrl-g were wired to the mode selector instead of area, or if it
// were wired to open the area popup on top of an already-open mode popup.
func TestExtractCtrlGInertWhileModePopupOpen(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)

	h.Send(ctrlF()) // open the mode popup
	if !h.Model().extractModePopupVisible() {
		t.Fatalf("expected mode popup open after ctrl-f")
	}
	beforeCategory := h.Model().extractCategory
	beforeArea := h.Model().extractGrabArea

	h.Send(ctrlG()) // must be inert while the mode popup is open

	if !h.Model().extractModePopupVisible() {
		t.Fatalf("ctrl-g while the mode popup is open must not close the mode popup")
	}
	if h.Model().extractAreaPopupVisible() {
		t.Fatalf("ctrl-g while the mode popup is open must not open the area popup")
	}
	if h.Model().extractGrabArea != beforeArea {
		t.Fatalf("ctrl-g while the mode popup is open must not change the area")
	}
	if h.Model().extractCategory != beforeCategory {
		t.Fatalf("ctrl-g while the mode popup is open must not change the category")
	}
}

func TestExtractCtrlFInertWhileAreaPopupOpen(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)

	h.Send(ctrlG()) // open the area popup
	if !h.Model().extractAreaPopupVisible() {
		t.Fatalf("expected area popup open after ctrl-g")
	}
	beforeCategory := h.Model().extractCategory
	beforeArea := h.Model().extractGrabArea

	h.Send(ctrlF()) // must be inert while the area popup is open

	if !h.Model().extractAreaPopupVisible() {
		t.Fatalf("ctrl-f while the area popup is open must not close the area popup")
	}
	if h.Model().extractModePopupVisible() {
		t.Fatalf("ctrl-f while the area popup is open must not open the mode popup")
	}
	if h.Model().extractCategory != beforeCategory {
		t.Fatalf("ctrl-f while the area popup is open must not change the category")
	}
	if h.Model().extractGrabArea != beforeArea {
		t.Fatalf("ctrl-f while the area popup is open must not change the area")
	}
}

// TestExtractCombinedBottomBarShowsModeAndArea verifies the single bottom-bar
// line renders both the mode and area segments with current values.
func TestExtractCombinedBottomBarShowsModeAndArea(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := ansi.Strip(h.View())
	if !strings.Contains(view, "mode: word <^f>") {
		t.Fatalf("expected mode segment 'mode: word <^f>', got:\n%s", view)
	}
	if !strings.Contains(view, "area: viewport <^g>") {
		t.Fatalf("expected area segment 'area: viewport <^g>', got:\n%s", view)
	}
	if !strings.Contains(view, "insert: <Enter>") {
		t.Fatalf("expected insert action hint 'insert: <Enter>', got:\n%s", view)
	}
	if !strings.Contains(view, "copy: <Tab>") {
		t.Fatalf("expected copy action hint 'copy: <Tab>', got:\n%s", view)
	}
}

// TestExtractAreaAnchorColMatchesAreaValueColumn verifies
// extractAreaAnchorCol() points at the exact column where the area value
// begins on the rendered subtitle line, so the area popup overlays directly
// under it (mirroring how the mode popup anchors under "mode: ").
func TestExtractAreaAnchorColMatchesAreaValueColumn(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "please make build", nil
	})
	defer restore()
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "x"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	lines := strings.Split(ansi.Strip(h.View()), "\n")
	var subtitleLine string
	for _, ln := range lines {
		if strings.Contains(ln, "mode:") {
			subtitleLine = ln
			break
		}
	}
	if subtitleLine == "" {
		t.Fatalf("could not find the extract subtitle line in the view")
	}
	idx := strings.Index(subtitleLine, "viewport")
	if idx < 0 {
		t.Fatalf("expected %q to contain the area value %q", subtitleLine, "viewport")
	}
	if got := h.Model().extractAreaAnchorCol(); got != idx {
		t.Fatalf("extractAreaAnchorCol() = %d, want %d (column of %q on the subtitle line %q)", got, idx, "viewport", subtitleLine)
	}
}
