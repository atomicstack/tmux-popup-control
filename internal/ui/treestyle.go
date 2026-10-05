package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
)

// TreeStyle names a connector style for the session tree.
type TreeStyle string

const (
	// TreeStyleClassic keeps the expand arrow in the label, detached from
	// the connector: "├─ ▶ claude".
	TreeStyleClassic TreeStyle = "classic"
	// TreeStyleArrow makes the arrow the connector's tip: "├─▶ claude".
	TreeStyleArrow TreeStyle = "arrow"
	// TreeStyleBox uses plus/minus boxes as the tip: "├─⊞ claude".
	TreeStyleBox TreeStyle = "box"
	// TreeStyleCompact drops the dash for small triangles: "├▸ claude".
	TreeStyleCompact TreeStyle = "compact"
	// TreeStyleRounded drops a tee into expanded children and rounds the
	// last child's corner: "├─┬ work" / "╰─▸ misc".
	TreeStyleRounded TreeStyle = "rounded"
)

// DefaultTreeStyle is used when no style is configured.
const DefaultTreeStyle = TreeStyleClassic

// treeStyleOrder is the cycle order for the tree style key.
var treeStyleOrder = []TreeStyle{
	TreeStyleClassic,
	TreeStyleArrow,
	TreeStyleBox,
	TreeStyleCompact,
	TreeStyleRounded,
}

// treeStyleSpec describes how one style draws connectors. The classic
// style has no spec: it keeps lipgloss's default padding and puts the
// expand arrow in the label.
type treeStyleSpec struct {
	branch, last        string // connector body for middle / last children
	collapsed, expanded string // tip for expandable nodes
	leaf                string // tip for nodes with no children
	indent, lastIndent  string // continuation under middle / last children
}

var treeStyleSpecs = map[TreeStyle]treeStyleSpec{
	TreeStyleArrow: {
		branch: "├─", last: "└─",
		collapsed: "▶", expanded: "▼", leaf: "─",
		indent: "│ ", lastIndent: "  ",
	},
	TreeStyleBox: {
		branch: "├─", last: "└─",
		collapsed: "⊞", expanded: "⊟", leaf: "─",
		indent: "│ ", lastIndent: "  ",
	},
	TreeStyleCompact: {
		branch: "├", last: "└",
		collapsed: "▸", expanded: "▾", leaf: "─",
		indent: "│", lastIndent: " ",
	},
	TreeStyleRounded: {
		branch: "├─", last: "╰─",
		collapsed: "▸", expanded: "┬", leaf: "─",
		indent: "│ ", lastIndent: "  ",
	},
}

// ParseTreeStyle resolves a configured style name. Empty input yields the
// default; unknown names report false so callers can warn.
func ParseTreeStyle(name string) (TreeStyle, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return DefaultTreeStyle, true
	}
	for _, s := range treeStyleOrder {
		if string(s) == name {
			return s, true
		}
	}
	return DefaultTreeStyle, false
}

// next returns the style after s in the cycle order.
func (s TreeStyle) next() TreeStyle {
	if s == "" {
		s = DefaultTreeStyle
	}
	for i, candidate := range treeStyleOrder {
		if candidate == s {
			return treeStyleOrder[(i+1)%len(treeStyleOrder)]
		}
	}
	return DefaultTreeStyle
}

// labelIndicators reports whether the style puts the expand arrow in the
// node label rather than in the connector.
func (s TreeStyle) labelIndicators() bool {
	_, ok := treeStyleSpecs[s]
	return !ok
}

// treeNodeStates records whether an expandable node is open, so the
// enumerator can pick its tip. Nodes absent from the map are leaves.
type treeNodeStates map[*tree.Tree]bool

// apply configures root's connectors for the style. Nested trees reuse
// the root's renderer, so this covers every depth.
func (s TreeStyle) apply(root *tree.Tree, states treeNodeStates) {
	spec, ok := treeStyleSpecs[s]
	if !ok {
		root.Enumerator(minimalEnumerator)
		return
	}
	plain := lipgloss.NewStyle()
	root.EnumeratorStyle(plain).IndenterStyle(plain)
	root.Enumerator(func(children tree.Children, index int) string {
		body := spec.branch
		if children.Length()-1 == index {
			body = spec.last
		}
		tip := spec.leaf
		if node, isTree := children.At(index).(*tree.Tree); isTree {
			if expanded, expandable := states[node]; expandable {
				tip = spec.collapsed
				if expanded {
					tip = spec.expanded
				}
			}
		}
		return body + tip + " "
	})
	root.Indenter(func(children tree.Children, index int) string {
		if children.Length()-1 == index {
			return spec.lastIndent
		}
		return spec.indent
	})
}

// treeStyleCycleKey cycles through the tree styles on tree levels.
const treeStyleCycleKey = "ctrl+t"

// resolveTreeStyle parses a configured style, logging and falling back to
// the default when the name is unknown.
func resolveTreeStyle(name string) TreeStyle {
	style, ok := ParseTreeStyle(name)
	if !ok {
		logging.Error(fmt.Errorf("unknown tree style %q, using %s", name, style))
	}
	return style
}

// cycleTreeStyle advances to the next tree style and names it in the info
// line.
func (m *Model) cycleTreeStyle() {
	m.treeStyle = m.treeStyle.next()
	m.setInfo("tree style: " + string(m.treeStyle))
}
