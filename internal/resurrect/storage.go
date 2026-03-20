package resurrect

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// tmuxOptionFn queries a tmux option value. Injectable for tests.
var tmuxOptionFn = func(socket, opt string) string {
	return tmux.ShowOption(socket, opt)
}

// withTmuxOptionFn replaces tmuxOptionFn for the duration of a test and
// returns a restore function.
func withTmuxOptionFn(fn func(socket, opt string) string) func() {
	orig := tmuxOptionFn
	tmuxOptionFn = fn
	return func() { tmuxOptionFn = orig }
}

// ResolveDir returns the directory used for save files. Lookup chain:
//  1. TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR env var
//  2. @tmux-popup-control-session-storage-dir tmux option
//  3. $XDG_DATA_HOME/tmux-popup-control-sessions/
//  4. $HOME/tmux-popup-control-sessions/
//
// The directory is created if it does not already exist.
func ResolveDir(socketPath string) (string, error) {
	if d := os.Getenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR"); d != "" {
		return ensureDir(os.ExpandEnv(d))
	}

	if d := tmuxOptionFn(socketPath, "@tmux-popup-control-session-storage-dir"); d != "" {
		return ensureDir(os.ExpandEnv(d))
	}

	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return ensureDir(filepath.Join(xdg, "tmux-popup-control-sessions"))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home directory: %w", err)
	}
	return ensureDir(filepath.Join(home, "tmux-popup-control-sessions"))
}

// ensureDir creates dir (and any parents) if it does not exist, then returns
// the path unchanged.
func ensureDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create storage directory %q: %w", dir, err)
	}
	return dir, nil
}

// savePath returns the full file path for a save file. If name is empty an
// auto-timestamped filename is used; otherwise name.json is used.
func savePath(dir, name string) string {
	if name == "" {
		ts := time.Now().Format("20060102T150405")
		return filepath.Join(dir, "save_"+ts+".json")
	}
	return filepath.Join(dir, name+".json")
}

// paneArchivePath returns the path for the pane-contents archive that
// accompanies a save file.
func paneArchivePath(jsonPath string) string {
	base := strings.TrimSuffix(jsonPath, ".json")
	return base + ".panes.tar.gz"
}

// updateLastSymlink points the "last" symlink in dir at target. The symlink is
// created atomically via a rename so readers never see a dangling link.
func updateLastSymlink(dir, target string) error {
	link := filepath.Join(dir, "last")
	tmp := link + ".tmp"
	// remove any stale tmp link
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("could not create symlink: %w", err)
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("could not update last symlink: %w", err)
	}
	return nil
}

// WriteSaveFile marshals sf to indented JSON and writes it to path.
func WriteSaveFile(path string, sf *SaveFile) error {
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal save file: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("could not write save file %q: %w", path, err)
	}
	return nil
}

// ReadSaveFile reads path and unmarshals its JSON into a SaveFile.
func ReadSaveFile(path string) (*SaveFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read save file %q: %w", path, err)
	}
	var sf SaveFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("could not parse save file %q: %w", path, err)
	}
	return &sf, nil
}

// LatestSave resolves the "last" symlink in dir and returns the absolute path
// of the most recent save file. Returns an error containing "no saved session
// found" when no symlink or target exists.
func LatestSave(dir string) (string, error) {
	link := filepath.Join(dir, "last")
	target, err := os.Readlink(link)
	if err != nil {
		return "", errors.New("no saved session found")
	}
	// target may be relative; resolve it relative to dir
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	if _, err := os.Stat(target); err != nil {
		return "", errors.New("no saved session found")
	}
	return target, nil
}

// ListSaves scans dir for *.json files (excluding the "last" symlink target
// if it appears separately), parses each, and returns them sorted newest-first.
func ListSaves(dir string) ([]SaveEntry, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("could not list save files: %w", err)
	}

	var entries []SaveEntry
	for _, p := range matches {
		sf, err := ReadSaveFile(p)
		if err != nil {
			// skip unreadable / malformed files
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		entries = append(entries, SaveEntry{
			Path:            p,
			Name:            sf.Name,
			Timestamp:       sf.Timestamp,
			HasPaneContents: sf.HasPaneContents,
			Size:            info.Size(),
			SessionCount:    len(sf.Sessions),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	return entries, nil
}

// ResolvePaneContents reports whether pane content capture is enabled.
// Lookup chain:
//  1. TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS env var
//  2. @tmux-popup-control-restore-pane-contents tmux option
//  3. false (default)
func ResolvePaneContents(socketPath string) bool {
	if v := os.Getenv("TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS"); v != "" {
		return parseBool(v)
	}
	if v := tmuxOptionFn(socketPath, "@tmux-popup-control-restore-pane-contents"); v != "" {
		return parseBool(v)
	}
	return false
}

// parseBool returns true for common truthy values.
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// SaveFileExists reports whether a named snapshot file exists in dir.
func SaveFileExists(dir, name string) bool {
	_, err := os.Stat(savePath(dir, name))
	return err == nil
}
