package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atomicstack/tmux-popup-control/internal/contentsearch"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// findHistoryKey toggles the "all history" tick box on a find level. It is an
// alt key because every fitting control key is taken by emacs-style editing.
const findHistoryKey = "alt+a"

// findPreviewContext is how many lines above the match the preview keeps.
const findPreviewContext = 3

// findCaptureFn captures one pane for the find preview; injectable for tests.
var findCaptureFn = menu.FindCapture

// findMatchStyle highlights the matched text inside the preview.
var findMatchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("220")).Bold(true)

// findState is the per-visit state of a find-by-content level, stored in the
// level's Data. Searches are single-flight: one runs at a time, and whatever
// the query has become by the time it lands is searched next (see
// findSyncCmd), so fast typing never queues a sweep per keystroke.
type findState struct {
	history bool // the "all history" tick box

	running    bool   // a search is in flight
	synced     bool   // a search for (query, reqHistory) has been issued
	query      string // query of the last issued search
	reqHistory bool   // history flag of the last issued search

	hasApplied     bool   // results below are for (applied, appliedHistory)
	applied        string // query the listed results belong to
	appliedHistory bool
	matches        map[string]menu.FindResult // item id → result
}

// findResultMsg carries a finished search back to the level that issued it.
// state identifies the visit, so a reply that outlives its level is dropped.
type findResultMsg struct {
	state   *findState
	query   string
	history bool
	results []menu.FindResult
	err     error
}

func isFindLevelID(id string) bool {
	return id == menu.PaneFindID || id == menu.WindowFindID
}

// findStateOf returns the find state of l, or nil when l is not a find level.
func findStateOf(l *level) *findState {
	if l == nil {
		return nil
	}
	st, _ := l.Data.(*findState)
	return st
}

// initFindLevel gives a freshly built find level its state and tick-box row.
func (m *Model) initFindLevel(l *level) {
	if l == nil || !isFindLevelID(l.ID) {
		return
	}
	st := &findState{}
	l.Data = st
	l.Subtitle = findSubtitle(l, st)
}

// handleFindKey handles the keys a find level owns. Everything else (typing,
// cursor movement, enter, esc) falls through to the normal menu handling.
func (m *Model) handleFindKey(key string) bool {
	st := findStateOf(m.currentLevel())
	if st == nil {
		return false
	}
	switch key {
	case findHistoryKey:
		st.history = !st.history
		return true
	case "tab":
		// No multi-select and nothing to complete on a search level.
		return true
	}
	return false
}

// findInvalidate marks a viewport search stale after the pane list changed,
// so the results follow live tmux state. History searches are left alone:
// re-reading every pane's scrollback on each backend update is too heavy, and
// they refresh whenever the query or the tick box changes.
func (m *Model) findInvalidate() {
	if st := findStateOf(m.currentLevel()); st != nil && !st.history {
		st.synced = false
	}
}

// findSyncCmd runs after every update. When the current level is a find level
// whose query or tick box differs from the last issued search, and no search
// is in flight, it starts one. It also keeps the tick-box row current.
func (m *Model) findSyncCmd() tea.Cmd {
	lvl := m.currentLevel()
	st := findStateOf(lvl)
	if st == nil || m.mode != ModeMenu {
		return nil
	}
	var cmd tea.Cmd
	if !m.loading && !st.running && (!st.synced || st.query != lvl.Filter || st.reqHistory != st.history) {
		cmd = m.findStartSearch(lvl, st)
	}
	lvl.Subtitle = findSubtitle(lvl, st)
	return cmd
}

func (m *Model) findStartSearch(lvl *level, st *findState) tea.Cmd {
	query, history := lvl.Filter, st.history
	st.synced, st.query, st.reqHistory = true, query, history
	if contentsearch.Blank(query) {
		return m.applyFindResults(lvl, st, query, history, nil)
	}
	st.running = true
	search := menu.FindPanes
	if lvl.ID == menu.WindowFindID {
		search = menu.FindWindows
	}
	ctx := m.menuContext()
	return func() tea.Msg {
		results, err := search(ctx, query, history)
		return findResultMsg{state: st, query: query, history: history, results: results, err: err}
	}
}

func (m *Model) handleFindResultMsg(msg tea.Msg) tea.Cmd {
	result, ok := msg.(findResultMsg)
	if !ok || result.state == nil {
		return nil
	}
	idx := slices.IndexFunc(m.stack, func(l *level) bool { return findStateOf(l) == result.state })
	if idx < 0 {
		return nil // the level this search belonged to is gone
	}
	lvl, st := m.stack[idx], result.state
	st.running = false
	if result.err != nil {
		m.errMsg = result.err.Error()
		return nil
	}
	m.errMsg = ""
	return m.applyFindResults(lvl, st, result.query, result.history, result.results)
}

// applyFindResults replaces the level's items with results, keeping the cursor
// on the same pane or window when it is still listed.
func (m *Model) applyFindResults(lvl *level, st *findState, query string, history bool, results []menu.FindResult) tea.Cmd {
	prevID := ""
	if lvl.Cursor >= 0 && lvl.Cursor < len(lvl.Items) {
		prevID = lvl.Items[lvl.Cursor].ID
	}
	items := make([]menu.Item, 0, len(results))
	st.matches = make(map[string]menu.FindResult, len(results))
	for _, r := range results {
		items = append(items, r.Item)
		st.matches[r.Item.ID] = r
	}
	st.hasApplied, st.applied, st.appliedHistory = true, query, history
	lvl.UpdateItems(items)
	lvl.Cursor = max(slices.IndexFunc(lvl.Items, func(it menu.Item) bool { return it.ID == prevID }), 0)
	m.syncViewport(lvl)
	lvl.Subtitle = findSubtitle(lvl, st)
	if lvl != m.currentLevel() {
		return nil
	}
	return m.ensurePreviewForLevel(lvl)
}

// findSubtitle renders the row above the search input: the "all history" tick
// box with its key hint, then the state of the search. It embeds ANSI, so it
// must be placed as a raw styledLine (see renderBottomBarLines).
func findSubtitle(lvl *level, st *findState) string {
	dim := styles.FilterPlaceholder
	box, boxStyle := "□", styles.Checkbox
	if st.history {
		box, boxStyle = "■", styles.CheckboxChecked
	}
	out := boxStyle.Render(box) + dim.Render(" all history <") + styles.SelectorHintKey.Render(findHistoryKey) + dim.Render(">")
	if status := findStatus(lvl, st); status != "" {
		out += dim.Render("   " + status)
	}
	return out
}

// findEmptyMessage is the list placeholder of a find level with no results.
func findEmptyMessage(lvl *level, st *findState) string {
	switch findStatus(lvl, st) {
	case "":
		return "(type to search pane contents)"
	case "no matches":
		return fmt.Sprintf("no matches for %q", lvl.Filter)
	default:
		return "(searching…)"
	}
}

func findStatus(lvl *level, st *findState) string {
	if contentsearch.Blank(lvl.Filter) {
		return ""
	}
	if !st.hasApplied || st.applied != lvl.Filter || st.appliedHistory != st.history {
		return "searching…"
	}
	n := len(lvl.Items)
	if n == 0 {
		return "no matches"
	}
	noun := "pane"
	if lvl.ID == menu.WindowFindID {
		noun = "window"
	}
	if n == 1 {
		return fmt.Sprintf("1 %s matches", noun)
	}
	return fmt.Sprintf("%d %ss match", n, noun)
}

// findPreviewCmd captures the pane behind a result the same way the search
// did (viewport or full history), highlights every occurrence of query, and
// asks the preview to scroll to the first one.
func findPreviewCmd(levelID, target, paneID, query string, history bool, seq int, socket string) tea.Cmd {
	return func() tea.Msg {
		msg := previewLoadedMsg{levelID: levelID, kind: previewKindFind, target: target, seq: seq}
		text, err := findCaptureFn(socket, paneID, history)
		if err != nil {
			msg.err = err
			return msg
		}
		lines, row := findPreviewLines(text, query)
		msg.lines = lines
		msg.rawANSI = true
		msg.scrollTo = max(row-findPreviewContext, 0)
		msg.scrollKey = fmt.Sprintf("%s\x00%t\x00%s", paneID, history, query)
		return msg
	}
}

// findPreviewLines splits captured text into styled preview lines with every
// occurrence of query highlighted, and returns the row of the first match
// (0 when there is none).
func findPreviewLines(text, query string) ([]string, int) {
	raw := strings.Split(strings.TrimRight(text, "\n"), "\n")
	lines := make([]string, len(raw))
	first := -1
	for i, line := range raw {
		spans := contentsearch.Spans(line, query)
		if len(spans) == 0 {
			lines[i] = renderFindPlain(line)
			continue
		}
		if first < 0 {
			first = i
		}
		var b strings.Builder
		pos := 0
		for _, span := range spans {
			b.WriteString(renderFindPlain(line[pos:span[0]]))
			b.WriteString(findMatchStyle.Render(line[span[0]:span[1]]))
			pos = span[1]
		}
		b.WriteString(renderFindPlain(line[pos:]))
		lines[i] = b.String()
	}
	return lines, max(first, 0)
}

func renderFindPlain(text string) string {
	if text == "" || styles.PreviewBody == nil {
		return text
	}
	return styles.PreviewBody.Render(text)
}
