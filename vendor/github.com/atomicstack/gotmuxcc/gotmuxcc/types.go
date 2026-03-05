// Package gotmuxcc exposes the public API surface compatible with the original
// github.com/GianlucaP106/gotmux module while adopting a control-mode backend.
package gotmuxcc

import "io"

// Socket references a tmux socket path.
type Socket struct {
	Path string
}

// Server contains global tmux server information.
type Server struct {
	Pid       int32
	Socket    *Socket
	StartTime string
	Uid       string
	User      string
	Version   string

	tmux *Tmux
}

// Client represents a tmux client.
type Client struct {
	Activity     string
	CellHeight   int
	CellWidth    int
	ControlMode  bool
	Created      string
	Discarded    string
	Flags        string
	Height       int
	KeyTable     string
	LastSession  string
	Name         string
	Pid          int32
	Prefix       bool
	Readonly     bool
	Session      string
	Termname     string
	Termfeatures string
	Termtype     string
	Tty          string
	Uid          int32
	User         string
	Utf8         bool
	Width        int
	Written      string

	tmux *Tmux
}

// Session represents a tmux session.
type Session struct {
	Activity          string
	Alerts            string
	Attached          int
	AttachedList      []string
	Created           string
	Format            bool
	Group             string
	GroupAttached     int
	GroupAttachedList []string
	GroupList         []string
	GroupManyAttached bool
	GroupSize         int
	Grouped           bool
	Id                string
	LastAttached      string
	ManyAttached      bool
	Marked            bool
	Name              string
	Path              string
	Stack             string
	Windows           int

	tmux *Tmux
}

// Window represents a tmux window.
type Window struct {
	Active             bool
	ActiveClients      int
	ActiveClientsList  []string
	ActiveSessions     int
	ActiveSessionsList []string
	Activity           string
	ActivityFlag       bool
	BellFlag           bool
	Bigger             bool
	CellHeight         int
	CellWidth          int
	EndFlag            bool
	Flags              string
	Format             bool
	Height             int
	Id                 string
	Index              int
	LastFlag           bool
	Layout             string
	Linked             bool
	LinkedSessions     int
	LinkedSessionsList []string
	MarkedFlag         bool
	Name               string
	Session            string
	OffsetX            int
	OffsetY            int
	Panes              int
	RawFlags           string
	SilenceFlag        int
	StackIndex         int
	StartFlag          bool
	VisibleLayout      string
	Width              int
	ZoomedFlag         bool

	tmux *Tmux
}

// Pane represents a tmux pane.
type Pane struct {
	Active         bool
	AtBottom       bool
	AtLeft         bool
	AtRight        bool
	AtTop          bool
	Bg             string
	Bottom         string
	CurrentCommand string
	CurrentPath    string
	Dead           bool
	DeadSignal     int
	DeadStatus     int
	DeadTime       string
	Fg             string
	Format         bool
	Height         int
	Id             string
	InMode         bool
	Index          int
	InputOff       bool
	Last           bool
	Left           string
	Marked         bool
	MarkedSet      bool
	Mode           string
	Path           string
	Pid            int32
	Pipe           bool
	Right          string
	SearchString   string
	SessionName    string
	StartCommand   string
	StartPath      string
	Synchronized   bool
	Tabs           string
	Title          string
	Top            string
	Tty            string
	UnseenChanges  bool
	Width          int
	WindowIndex    int

	tmux *Tmux
}

// Option models a tmux option key/value pair.
type Option struct {
	Key   string
	Value string
}

// WindowLayout enumerates tmux window layouts.
type WindowLayout string

const (
	WindowLayoutEvenHorizontal WindowLayout = "even-horizontal"
	WindowLayoutEvenVertical   WindowLayout = "even-vertical"
	WindowLayoutMainVertical   WindowLayout = "main-horizontal"
	WindowLayoutTiled          WindowLayout = "tiled"
)

// PanePosition enumerates select-pane targets.
type PanePosition string

const (
	PanePositionUp    PanePosition = "-U"
	PanePositionRight PanePosition = "-R"
	PanePositionDown  PanePosition = "-D"
	PanePositionLeft  PanePosition = "-L"
)

// PaneSplitDirection enumerates split-window directions.
type PaneSplitDirection string

const (
	PaneSplitDirectionHorizontal PaneSplitDirection = "-h"
	PaneSplitDirectionVertical   PaneSplitDirection = "-v"
)

// SessionOptions configures new session creation.
type SessionOptions struct {
	Name           string
	ShellCommand   string
	StartDirectory string
	Width          int
	Height         int
}

// DetachClientOptions customises detach-client behavior.
type DetachClientOptions struct {
	TargetClient  string
	TargetSession string
}

// SwitchClientOptions customises switch-client behavior.
type SwitchClientOptions struct {
	TargetSession string
	TargetClient  string
}

// AttachSessionOptions customises attach-session behavior.
type AttachSessionOptions struct {
	WorkingDir    string
	DetachClients bool

	Output io.Writer
	Error  io.Writer
}

// NewWindowOptions customises new-window behavior.
type NewWindowOptions struct {
	StartDirectory string
	WindowName     string
	DoNotAttach    bool
}

// SelectPaneOptions customises select-pane behavior.
type SelectPaneOptions struct {
	TargetPosition PanePosition
}

// SplitWindowOptions customises split-window behavior.
type SplitWindowOptions struct {
	SplitDirection PaneSplitDirection
	StartDirectory string
	ShellCommand   string
}

// ChooseTreeOptions customises choose-tree behavior.
type ChooseTreeOptions struct {
	SessionsCollapsed bool
	WindowsCollapsed  bool
}

// CaptureOptions customises capture-pane behavior.
type CaptureOptions struct {
	EscTxtNBgAttr    bool
	EscNonPrintables bool
	IgnoreTrailing   bool
	PreserveTrailing bool
	PreserveAndJoin  bool
	StartLine        string // -S flag (e.g. "-40", "0", "-"); empty means default
	EndLine          string // -E flag (e.g. "-1", "-"); empty means default
}

// ResizeDirection enumerates resize-pane directions.
type ResizeDirection string

const (
	ResizeLeft  ResizeDirection = "-L"
	ResizeRight ResizeDirection = "-R"
	ResizeUp    ResizeDirection = "-U"
	ResizeDown  ResizeDirection = "-D"
)
