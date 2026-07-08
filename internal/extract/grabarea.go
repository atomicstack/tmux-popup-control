package extract

// GrabArea selects which region of pane/window content a grab captures.
type GrabArea int

const (
	Viewport GrabArea = iota
	PaneHistory
	Window
	WindowHistory
)

// DefaultGrabArea is the grab area shown when the extract view first opens.
const DefaultGrabArea = Viewport

// grabAreaOrder is the cycle order.
var grabAreaOrder = []GrabArea{Viewport, PaneHistory, Window, WindowHistory}

// GrabAreas returns the grab-area cycle order
// (viewport→pane-history→window→window-history).
// Callers get a copy, so mutating the returned slice cannot corrupt the
// package-level cycle order used by Next().
func GrabAreas() []GrabArea { return append([]GrabArea(nil), grabAreaOrder...) }

func (a GrabArea) String() string {
	switch a {
	case Viewport:
		return "viewport"
	case PaneHistory:
		return "pane-history"
	case Window:
		return "window"
	case WindowHistory:
		return "window-history"
	default:
		return "viewport"
	}
}

// Next returns the next grab area in cycle order, wrapping after
// WindowHistory.
func (a GrabArea) Next() GrabArea {
	for i, o := range grabAreaOrder {
		if o == a {
			return grabAreaOrder[(i+1)%len(grabAreaOrder)]
		}
	}
	return DefaultGrabArea
}
