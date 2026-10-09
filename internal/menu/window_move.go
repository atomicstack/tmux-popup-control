package menu

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

var moveWindowToFn = tmux.MoveWindowTo

// WindowMoveMarker prefixes the label of the window being moved so the
// pinned row stands out from its neighbours.
const WindowMoveMarker = "⇅ "

// windowMoveSlot is one position the moving window can occupy: the gap before
// the pos'th remaining window of a session (pos == len means after the last).
type windowMoveSlot struct {
	session int
	pos     int
}

// WindowMoveState drives the interactive window:move tree. The window being
// moved is pinned to the cursor and walks through every slot between the
// windows of every session as the user presses up/down.
type WindowMoveState struct {
	source   WindowEntry
	sessions []SessionEntry
	// others holds each session's windows, in order, minus the source.
	others [][]WindowEntry
	slots  []windowMoveSlot
	slot   int
	origin int
}

// NewWindowMoveState builds the move tree from the context's sessions and
// windows and pins the current window at its present position. ok is false
// when there is no current window to move.
func NewWindowMoveState(ctx Context) (*WindowMoveState, bool) {
	source, ok := windowMoveSource(ctx)
	if !ok {
		return nil, false
	}
	return buildWindowMoveState(ctx, source)
}

// buildWindowMoveState lays out the slots for ctx with source pinned at its
// own position. ok is false when source is not among ctx's windows.
func buildWindowMoveState(ctx Context, source WindowEntry) (*WindowMoveState, bool) {
	s := &WindowMoveState{source: source, sessions: ctx.Sessions}
	s.others = make([][]WindowEntry, len(ctx.Sessions))
	origin := -1
	for i, sess := range ctx.Sessions {
		var windows []WindowEntry
		found := false
		for _, w := range ctx.Windows {
			if w.Session != sess.Name {
				continue
			}
			if w.Session == source.Session && sameWindow(w, source) {
				s.source = w
				found = true
				continue
			}
			windows = append(windows, w)
		}
		sortWindowEntries(windows)
		if found {
			pos := 0
			for pos < len(windows) && windows[pos].Index < s.source.Index {
				pos++
			}
			origin = len(s.slots) + pos
		}
		s.others[i] = windows
		for pos := 0; pos <= len(windows); pos++ {
			s.slots = append(s.slots, windowMoveSlot{session: i, pos: pos})
		}
	}
	if origin < 0 {
		return nil, false
	}
	s.origin = origin
	s.slot = origin
	return s, true
}

// Refresh rebuilds the tree from fresh backend data, keeping the same window
// pinned. An unmoved window follows its real position; a moved one stays
// anchored after the same window above it (or before the same window below
// it), falling back to the same position in the same session. ok is false
// when the pinned window no longer exists.
func (s *WindowMoveState) Refresh(ctx Context) bool {
	next, ok := buildWindowMoveState(ctx, s.source)
	if !ok {
		return false
	}
	if s.Moved() {
		next.slot = next.anchoredSlot(s)
	}
	*s = *next
	return true
}

// anchoredSlot finds the slot in s matching prev's current slot.
func (s *WindowMoveState) anchoredSlot(prev *WindowMoveState) int {
	slot := prev.slots[prev.slot]
	prevSession := prev.sessions[slot.session]
	prevWindows := prev.others[slot.session]
	sess := slices.IndexFunc(s.sessions, func(e SessionEntry) bool {
		if e.ID != "" && prevSession.ID != "" {
			return e.ID == prevSession.ID
		}
		return e.Name == prevSession.Name
	})
	if sess < 0 {
		return s.origin
	}
	windows := s.others[sess]
	pos := min(slot.pos, len(windows))
	if slot.pos > 0 {
		if i := slices.IndexFunc(windows, func(w WindowEntry) bool { return sameWindow(w, prevWindows[slot.pos-1]) }); i >= 0 {
			pos = i + 1
		}
	} else if slot.pos < len(prevWindows) {
		if i := slices.IndexFunc(windows, func(w WindowEntry) bool { return sameWindow(w, prevWindows[slot.pos]) }); i >= 0 {
			pos = i
		}
	}
	return slices.Index(s.slots, windowMoveSlot{session: sess, pos: pos})
}

func windowMoveSource(ctx Context) (WindowEntry, bool) {
	id := strings.TrimSpace(ctx.CurrentWindowID)
	if id != "" {
		if w, ok := ctx.WindowEntryFor(id); ok {
			return w, true
		}
	}
	for _, w := range ctx.Windows {
		if w.Current && w.Session == ctx.CurrentWindowSession {
			return w, true
		}
	}
	return WindowEntry{}, false
}

func sameWindow(a, b WindowEntry) bool {
	if a.InternalID != "" && b.InternalID != "" {
		return a.InternalID == b.InternalID
	}
	return a.ID == b.ID
}

// Source returns the window being moved.
func (s *WindowMoveState) Source() WindowEntry { return s.source }

// Moved reports whether the pinned window has left its original slot.
func (s *WindowMoveState) Moved() bool { return s.slot != s.origin }

// Step moves the pinned window delta slots (negative is up), clamped to the
// first/last slot. It reports whether the position changed.
func (s *WindowMoveState) Step(delta int) bool {
	next := min(max(s.slot+delta, 0), len(s.slots)-1)
	if next == s.slot {
		return false
	}
	s.slot = next
	return true
}

// StepHome / StepEnd jump to the first / last slot.
func (s *WindowMoveState) StepHome() bool { return s.Step(-len(s.slots)) }
func (s *WindowMoveState) StepEnd() bool  { return s.Step(len(s.slots)) }

// WindowMovePlan is the move-window invocation that realises a slot.
type WindowMovePlan struct {
	Source        string // tmux id of the moving window
	Target        string // "$S:idx" for an exact index, else a neighbour's @N
	Placement     tmux.WindowPlacement
	TargetSession string // display name, for messages
	Index         int    // index the window lands on
}

// Plan resolves the current slot into a move-window call. A free index right
// after the window above is targeted exactly so no other window is
// renumbered; otherwise the window is inserted after the window above (or
// before the window below, at the top of a session) and tmux shuffles the
// rest up.
func (s *WindowMoveState) Plan() WindowMovePlan {
	slot := s.slots[s.slot]
	sess := s.sessions[slot.session]
	windows := s.others[slot.session]
	plan := WindowMovePlan{
		Source:        windowTmuxTarget(s.source),
		TargetSession: sess.Name,
	}
	if slot.pos > 0 {
		above := windows[slot.pos-1]
		idx := above.Index + 1
		plan.Index = idx
		if !windowIndexTaken(windows, idx) {
			plan.Target = fmt.Sprintf("%s:%d", sessionTmuxTarget(sess), idx)
			plan.Placement = tmux.PlaceAt
			return plan
		}
		plan.Target = windowTmuxTarget(above)
		plan.Placement = tmux.PlaceAfter
		return plan
	}
	if len(windows) == 0 {
		// Only the source's own session can be left empty, and its single
		// slot is the origin, so this is a no-op move to the same session.
		plan.Target = sessionTmuxTarget(sess) + ":"
		plan.Placement = tmux.PlaceAt
		plan.Index = s.source.Index
		return plan
	}
	below := windows[0]
	plan.Target = windowTmuxTarget(below)
	plan.Placement = tmux.PlaceBefore
	plan.Index = below.Index
	return plan
}

func windowIndexTaken(windows []WindowEntry, idx int) bool {
	for _, w := range windows {
		if w.Index == idx {
			return true
		}
	}
	return false
}

func windowTmuxTarget(w WindowEntry) string {
	if w.InternalID != "" {
		return w.InternalID
	}
	return w.ID
}

func sessionTmuxTarget(s SessionEntry) string {
	if s.ID != "" {
		return s.ID
	}
	return s.Name
}

// TreeInput returns the sessions and windows to render, with the moving
// window spliced into its current slot. Windows that tmux would shuffle up to
// make room are shown at their new indices. The spliced entry carries a
// pre-formatted label (marker plus the index it would land on), and an Index
// of -1 so TreeWindowLabel leaves that label alone.
func (s *WindowMoveState) TreeInput() TreeItemsInput {
	slot := s.slots[s.slot]
	ghost := s.ghost()
	shifted := s.shiftedIndices()
	var windows []WindowEntry
	for i, others := range s.others {
		for pos, w := range others {
			if i == slot.session && pos == slot.pos {
				windows = append(windows, ghost)
			}
			if idx, ok := shifted[TreeWindowKey(w)]; ok && i == slot.session {
				w = relabelWindow(w, idx)
			}
			windows = append(windows, w)
		}
		if i == slot.session && slot.pos == len(others) {
			windows = append(windows, ghost)
		}
	}
	return TreeItemsInput{Sessions: s.sessions, Windows: windows}
}

// shiftedIndices mirrors tmux's winlink_shuffle_up for an -a/-b move: the
// contiguous run of windows starting at the landing index moves up by one.
// The source still occupies its old index while tmux shuffles, so it counts
// towards the run when it lives in the target session.
func (s *WindowMoveState) shiftedIndices() map[string]int {
	if !s.Moved() {
		return nil
	}
	plan := s.Plan()
	if plan.Placement == tmux.PlaceAt {
		return nil
	}
	slot := s.slots[s.slot]
	windows := s.others[slot.session]
	if s.sessions[slot.session].Name == s.source.Session {
		windows = append(slices.Clone(windows), s.source)
	}
	last := plan.Index
	for windowIndexTaken(windows, last) {
		last++
	}
	shifted := make(map[string]int)
	for _, w := range windows {
		if w.Index >= plan.Index && w.Index < last {
			shifted[TreeWindowKey(w)] = w.Index + 1
		}
	}
	return shifted
}

// relabelWindow renumbers a window entry, rewriting its default
// "session:index: rest" label. Labels in a custom format are left alone since
// their index cannot be located reliably.
func relabelWindow(w WindowEntry, idx int) WindowEntry {
	prefix := fmt.Sprintf("%s:%d: ", w.Session, w.Index)
	if !strings.HasPrefix(w.Label, prefix) {
		return w
	}
	w.Label = fmt.Sprintf("%s:%d: %s", w.Session, idx, w.Label[len(prefix):])
	w.Index = idx
	return w
}

func (s *WindowMoveState) ghost() WindowEntry {
	ghost := s.source
	ghost.Session = s.sessions[s.slots[s.slot].session].Name
	ghost.Current = false
	index := s.source.Index
	if s.Moved() {
		index = s.Plan().Index
	}
	rest := TreeWindowLabel(s.source)
	rest = strings.TrimPrefix(rest, fmt.Sprintf("%d: ", s.source.Index))
	ghost.Label = fmt.Sprintf("%s%d: %s", WindowMoveMarker, index, rest)
	ghost.Index = -1
	return ghost
}

// Items returns the flat tree item list for the current slot.
func (s *WindowMoveState) Items() []Item {
	return NewTreeState(true).BuildTreeItems(s.TreeInput())
}

// Cursor returns the index of the pinned window within Items.
func (s *WindowMoveState) Cursor() int {
	id := TreeWindowID(TreeWindowKey(s.ghost()))
	for i, it := range s.Items() {
		if it.ID == id {
			return i
		}
	}
	return 0
}

func loadWindowMoveMenu(ctx Context) ([]Item, error) {
	state, ok := NewWindowMoveState(ctx)
	if !ok {
		return nil, nil
	}
	return state.Items(), nil
}

// WindowMoveCommand executes a planned window move.
func WindowMoveCommand(socketPath string, plan WindowMovePlan) tea.Cmd {
	return func() tea.Msg {
		events.Window.Move(plan.Source, plan.Target)
		if err := moveWindowToFn(socketPath, plan.Source, plan.Target, plan.Placement); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Moved window to %s:%d", plan.TargetSession, plan.Index)}
	}
}
