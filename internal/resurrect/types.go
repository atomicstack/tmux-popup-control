package resurrect

import "time"

const currentVersion = 1

// Config is passed to Save/Restore by the caller.
type Config struct {
	SocketPath          string
	SaveDir             string // resolved by caller via ResolveDir()
	CapturePaneContents bool
	Name                string // empty for auto-timestamped
	ClientID            string // terminal client name for switch-client
}

// SaveFile is the top-level JSON structure written to disk.
type SaveFile struct {
	Version           int       `json:"version"`
	Timestamp         time.Time `json:"timestamp"`
	Name              string    `json:"name"`
	HasPaneContents   bool      `json:"has_pane_contents"`
	ClientSession     string    `json:"client_session"`
	ClientLastSession string    `json:"client_last_session"`
	Sessions          []Session `json:"sessions"`
}

// Session represents one tmux session in the save file.
type Session struct {
	Name     string   `json:"name"`
	Created  int64    `json:"created"`
	Attached bool     `json:"attached"`
	Windows  []Window `json:"windows"`
}

// Window represents one tmux window in the save file.
type Window struct {
	Index           int    `json:"index"`
	Name            string `json:"name"`
	Layout          string `json:"layout"`
	Active          bool   `json:"active"`
	Alternate       bool   `json:"alternate"`
	AutomaticRename bool   `json:"automatic_rename"`
	Panes           []Pane `json:"panes"`
}

// Pane represents one tmux pane in the save file.
type Pane struct {
	Index      int    `json:"index"`
	WorkingDir string `json:"working_dir"`
	Title      string `json:"title"`
	Command    string `json:"command"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Active     bool   `json:"active"`
}

// ProgressEvent is sent on the channel during save/restore.
type ProgressEvent struct {
	Step    int
	Total   int
	Message string
	Kind    string // "session", "window", "pane", "info", "error"
	ID      string // entity name/ID for UI colouring
	Done    bool
	Err     error
}

// SaveEntry represents one save file in the listing.
type SaveEntry struct {
	Path            string
	Name            string // snapshot name or empty for auto
	Timestamp       time.Time
	HasPaneContents bool
	Size            int64
	SessionCount    int
}
