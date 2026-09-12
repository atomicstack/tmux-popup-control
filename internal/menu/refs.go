package menu

import (
	"fmt"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// Menu items carry human-readable display ids ("session:index",
// "session:index.pane") because that is what users see and filter on. tmux
// next-3.8 allows ':' and '.' in session and window names, so those display
// ids can no longer be parsed back into tmux targets. The helpers below
// resolve a display id to the entry the context already holds and hand back
// the tmux id ($N, @N, %N) that every tmux command should target. When an
// entry is unknown the input is returned unchanged so callers that already
// hold a tmux id keep working.

// SessionEntryFor finds a session by name or tmux id.
func (ctx Context) SessionEntryFor(nameOrID string) (SessionEntry, bool) {
	key := strings.TrimSpace(nameOrID)
	if key == "" {
		return SessionEntry{}, false
	}
	for _, s := range ctx.Sessions {
		if s.Name == key || (s.ID != "" && s.ID == key) {
			return s, true
		}
	}
	return SessionEntry{}, false
}

// WindowEntryFor finds a window by display id or tmux id.
func (ctx Context) WindowEntryFor(displayOrID string) (WindowEntry, bool) {
	key := strings.TrimSpace(displayOrID)
	if key == "" {
		return WindowEntry{}, false
	}
	for _, w := range ctx.Windows {
		if w.ID == key || (w.InternalID != "" && w.InternalID == key) {
			return w, true
		}
	}
	return WindowEntry{}, false
}

// PaneEntryFor finds a pane by display id or tmux id.
func (ctx Context) PaneEntryFor(displayOrID string) (PaneEntry, bool) {
	key := strings.TrimSpace(displayOrID)
	if key == "" {
		return PaneEntry{}, false
	}
	for _, p := range ctx.Panes {
		if p.ID == key || (p.PaneID != "" && p.PaneID == key) {
			return p, true
		}
	}
	return PaneEntry{}, false
}

// sessionTarget returns the tmux target for a session name: its $N id when
// the context knows it, otherwise the name itself.
func (ctx Context) sessionTarget(nameOrID string) string {
	key := strings.TrimSpace(nameOrID)
	if s, ok := ctx.SessionEntryFor(key); ok && s.ID != "" {
		return s.ID
	}
	return key
}

// windowTarget returns the tmux target for a window display id: its @N id
// when the context knows it, otherwise the input.
func (ctx Context) windowTarget(displayOrID string) string {
	key := strings.TrimSpace(displayOrID)
	if w, ok := ctx.WindowEntryFor(key); ok && w.InternalID != "" {
		return w.InternalID
	}
	return key
}

// windowSessionTarget returns the tmux target for the session a window
// belongs to.
func (ctx Context) windowSessionTarget(w WindowEntry) string {
	if w.SessionID != "" {
		return w.SessionID
	}
	return ctx.sessionTarget(w.Session)
}

// paneTarget returns the tmux target for a pane display id: its %N id when
// the context knows it, otherwise the input.
func (ctx Context) paneTarget(displayOrID string) string {
	key := strings.TrimSpace(displayOrID)
	if p, ok := ctx.PaneEntryFor(key); ok && p.PaneID != "" {
		return p.PaneID
	}
	return key
}

// paneRef builds the id triple SwitchPane needs for a pane display id.
func (ctx Context) paneRef(displayOrID string) (tmux.PaneRef, bool) {
	p, ok := ctx.PaneEntryFor(displayOrID)
	if !ok {
		return tmux.PaneRef{}, false
	}
	ref := tmux.PaneRef{SessionID: p.SessionID, WindowID: p.WindowID, PaneID: p.PaneID}
	if ref.SessionID == "" {
		if s, ok := ctx.SessionEntryFor(p.Session); ok {
			ref.SessionID = s.ID
		}
	}
	if ref.WindowID == "" {
		if w, ok := ctx.WindowEntryFor(fmt.Sprintf("%s:%d", p.Session, p.WindowIdx)); ok {
			ref.WindowID = w.InternalID
		}
	}
	if ref.PaneID == "" {
		ref.PaneID = p.ID
	}
	return ref, true
}

// targetsFor maps a list of display ids through resolve, preserving order.
func targetsFor(ids []string, resolve func(string) string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, resolve(id))
	}
	return out
}
