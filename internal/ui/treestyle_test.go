package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// treeStyleFixture builds the sample tree used to compare styles: "work"
// and its "1: shell" window are expanded, everything else is collapsed.
func treeStyleFixture() ([]menu.SessionEntry, []menu.WindowEntry, []menu.PaneEntry, *menu.TreeState) {
	sessions := []menu.SessionEntry{
		{Name: "claude"},
		{Name: "work", Current: true},
		{Name: "misc"},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0: editor", Session: "work", Index: 0},
		{ID: "1", Label: "1: shell", Session: "work", Index: 1, Current: true},
		{ID: "2", Label: "2: logs", Session: "work", Index: 2},
	}
	panes := []menu.PaneEntry{
		{ID: "%4", PaneID: "%4", Label: "zsh", Session: "work", WindowIdx: 1, Index: 0},
		{ID: "%5", PaneID: "%5", Label: "htop", Session: "work", WindowIdx: 1, Index: 1},
	}
	ts := menu.NewTreeState(false)
	ts.SetExpanded(menu.TreeSessionID(menu.TreeSessionKey(sessions[1])), true)
	ts.SetExpanded(menu.TreeWindowID(menu.TreeWindowKey(windows[1])), true)
	return sessions, windows, panes, ts
}

func renderTreeStyleFixture(style TreeStyle) string {
	sessions, windows, panes, ts := treeStyleFixture()
	windowCounts := map[string]int{"claude": 31, "work": 3, "misc": 2}
	paneCounts := map[string]int{"work\x000": 1, "work\x001": 2, "work\x002": 1}
	return buildTree(sessions, windows, panes, ts, windowCounts, paneCounts, style).String()
}

func TestTreeStylesRenderConnectors(t *testing.T) {
	cases := map[TreeStyle]string{
		TreeStyleClassic: `
├─ ▶ claude (31 windows)
├─ ▼ work (3 windows) (current)
│   ├─ ▶ 0: editor (1 pane)
│   ├─ ▼ 1: shell (2 panes) (current)
│   │   ├─ 0: zsh
│   │   └─ 1: htop
│   └─ ▶ 2: logs (1 pane)
└─ ▶ misc (2 windows)`,
		TreeStyleArrow: `
├─▶ claude (31 windows)
├─▼ work (3 windows) (current)
│ ├─▶ 0: editor (1 pane)
│ ├─▼ 1: shell (2 panes) (current)
│ │ ├── 0: zsh
│ │ └── 1: htop
│ └─▶ 2: logs (1 pane)
└─▶ misc (2 windows)`,
		TreeStyleBox: `
├─⊞ claude (31 windows)
├─⊟ work (3 windows) (current)
│ ├─⊞ 0: editor (1 pane)
│ ├─⊟ 1: shell (2 panes) (current)
│ │ ├── 0: zsh
│ │ └── 1: htop
│ └─⊞ 2: logs (1 pane)
└─⊞ misc (2 windows)`,
		TreeStyleCompact: `
├▸ claude (31 windows)
├▾ work (3 windows) (current)
│├▸ 0: editor (1 pane)
│├▾ 1: shell (2 panes) (current)
││├─ 0: zsh
││└─ 1: htop
│└▸ 2: logs (1 pane)
└▸ misc (2 windows)`,
		TreeStyleRounded: `
├─▸ claude (31 windows)
├─┬ work (3 windows) (current)
│ ├─▸ 0: editor (1 pane)
│ ├─┬ 1: shell (2 panes) (current)
│ │ ├── 0: zsh
│ │ ╰── 1: htop
│ ╰─▸ 2: logs (1 pane)
╰─▸ misc (2 windows)`,
	}
	for _, style := range treeStyleOrder {
		t.Run(string(style), func(t *testing.T) {
			want := strings.TrimPrefix(cases[style], "\n")
			got := trimTrailingSpaces(renderTreeStyleFixture(style))
			if got != want {
				t.Fatalf("style %s rendered:\n%s\nwant:\n%s", style, got, want)
			}
		})
	}
}

func trimTrailingSpaces(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

func TestParseTreeStyle(t *testing.T) {
	cases := []struct {
		in   string
		want TreeStyle
		ok   bool
	}{
		{"", TreeStyleClassic, true},
		{"arrow", TreeStyleArrow, true},
		{" Rounded ", TreeStyleRounded, true},
		{"bogus", TreeStyleClassic, false},
	}
	for _, tc := range cases {
		got, ok := ParseTreeStyle(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseTreeStyle(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestTreeStyleCycleKeyAdvancesAndWraps(t *testing.T) {
	sessions, windows, panes, _ := treeStyleFixture()
	m := testTreeModel(sessions, windows, panes, false)
	if m.treeStyle != TreeStyleClassic {
		t.Fatalf("expected default style classic, got %q", m.treeStyle)
	}
	for _, want := range append(treeStyleOrder[1:], TreeStyleClassic) {
		m.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
		if m.treeStyle != want {
			t.Fatalf("after ctrl+t expected %q, got %q", want, m.treeStyle)
		}
		if got := m.currentInfo(); got != "tree style: "+string(want) {
			t.Fatalf("expected info %q, got %q", "tree style: "+string(want), got)
		}
	}
	m.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if view := m.View().Content; !strings.Contains(view, "├─▶ claude") {
		t.Fatalf("expected arrow style in view after cycling, got:\n%s", view)
	}
}

func TestTreeStyleCycleKeyIgnoredOutsideTree(t *testing.T) {
	m := NewModel(ModelConfig{TreeStyle: "box"})
	m.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if m.treeStyle != TreeStyleBox {
		t.Fatalf("expected style unchanged outside tree, got %q", m.treeStyle)
	}
}

func TestTreeStyleFromModelConfig(t *testing.T) {
	if got := NewModel(ModelConfig{TreeStyle: "compact"}).treeStyle; got != TreeStyleCompact {
		t.Fatalf("expected compact, got %q", got)
	}
	if got := NewModel(ModelConfig{TreeStyle: "nope"}).treeStyle; got != TreeStyleClassic {
		t.Fatalf("expected fallback to classic, got %q", got)
	}
}
