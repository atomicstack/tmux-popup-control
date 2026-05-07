package resurrect

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

type StorageDeps struct {
	ShowOption func(socket, opt string) string
}

var storageDeps = StorageDeps{
	ShowOption: tmux.ShowOption,
}

// withTmuxOptionFn replaces tmuxOptionFn for the duration of a test and
// returns a restore function.
func withTmuxOptionFn(fn func(socket, opt string) string) func() {
	orig := storageDeps.ShowOption
	storageDeps.ShowOption = fn
	return func() { storageDeps.ShowOption = orig }
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

	if d := storageDeps.ShowOption(socketPath, "@tmux-popup-control-session-storage-dir"); d != "" {
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
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("could not create storage directory %q: %w", dir, err)
	}
	return dir, nil
}

// ValidateSaveName rejects names containing path separators, leading dots, or
// glob metacharacters that could cause path traversal or unexpected matches.
func ValidateSaveName(name string) error {
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("name must not contain path separators")
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("name must not start with '.'")
	}
	if strings.ContainsAny(name, "*?[") {
		return fmt.Errorf("name must not contain glob characters")
	}
	return nil
}

// savePath returns the full file path for a save file. Unnamed saves use a
// UUID identifier with a timestamp; named saves include a timestamp suffix.
func savePath(dir, name string) string {
	ts := time.Now().Format("20060102T150405")
	if name == "" {
		id := uuid.New().String()
		return filepath.Join(dir, id+"_"+ts+".json")
	}
	return filepath.Join(dir, name+"_"+ts+".json")
}

// paneArchivePath returns the path for the pane-contents archive that
// accompanies a save file.
func paneArchivePath(jsonPath string) string {
	base := strings.TrimSuffix(jsonPath, ".json")
	return base + ".panes.tar.gz"
}

// DeleteSave removes the save file at path along with its companion
// pane-contents archive (if any). When the deleted file is the current
// "last" symlink target, the symlink is repointed at the newest
// remaining save, or removed when none remain. dir may be empty to
// skip the symlink fix-up (caller already knows there is none).
func DeleteSave(dir, path string) error {
	if path == "" {
		return errors.New("empty save path")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing save %q: %w", path, err)
	}
	archive := paneArchivePath(path)
	if err := os.Remove(archive); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing pane archive %q: %w", archive, err)
	}
	if dir == "" {
		return nil
	}
	// Detect a now-dangling "last" symlink and either repoint it at the
	// newest remaining save or drop it entirely.
	if _, err := LatestSave(dir); err == nil {
		return nil
	}
	link := filepath.Join(dir, "last")
	entries, lerr := ListSaves(dir)
	if lerr == nil && len(entries) > 0 {
		if err := updateLastSymlink(dir, filepath.Base(entries[0].Path)); err != nil {
			return fmt.Errorf("updating last symlink: %w", err)
		}
		return nil
	}
	if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing dangling last symlink: %w", err)
	}
	return nil
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
	if err := os.WriteFile(path, data, 0o600); err != nil {
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
	if sf.Kind == "" {
		sf.Kind = SaveKindManual
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
		var windows, panes int
		for _, s := range sf.Sessions {
			windows += len(s.Windows)
			for _, w := range s.Windows {
				panes += len(w.Panes)
			}
		}
		entries = append(entries, SaveEntry{
			Path:            p,
			Name:            sf.Name,
			Kind:            sf.Kind,
			Timestamp:       sf.Timestamp,
			HasPaneContents: sf.HasPaneContents,
			Size:            info.Size(),
			SessionCount:    len(sf.Sessions),
			WindowCount:     windows,
			PaneCount:       panes,
		})
	}

	slices.SortFunc(entries, func(a, b SaveEntry) int {
		return b.Timestamp.Compare(a.Timestamp)
	})
	return entries, nil
}

const (
	envAutosaveIntervalMinutes = "TMUX_POPUP_CONTROL_AUTOSAVE_INTERVAL_MINUTES"
	envAutosaveMax             = "TMUX_POPUP_CONTROL_AUTOSAVE_MAX"
	envAutosaveIcon            = "TMUX_POPUP_CONTROL_AUTOSAVE_ICON"
	envAutosaveIconSeconds     = "TMUX_POPUP_CONTROL_AUTOSAVE_ICON_SECONDS"
	optAutosaveIntervalMinutes = "@tmux-popup-control-autosave-interval-minutes"
	optAutosaveMax             = "@tmux-popup-control-autosave-max"
	optAutosaveIcon            = "@tmux-popup-control-autosave-icon"
	optAutosaveIconSeconds     = "@tmux-popup-control-autosave-icon-seconds"
)

func ResolveAutosaveIntervalMinutes(socketPath string) int {
	if v := strings.TrimSpace(os.Getenv(envAutosaveIntervalMinutes)); v != "" {
		return parseAutosaveInterval(v)
	}
	if v := strings.TrimSpace(storageDeps.ShowOption(socketPath, optAutosaveIntervalMinutes)); v != "" {
		return parseAutosaveInterval(v)
	}
	return 0
}

func ResolveAutosaveMax(socketPath string) int {
	if v := strings.TrimSpace(os.Getenv(envAutosaveMax)); v != "" {
		return parseAutosaveMax(v)
	}
	if v := strings.TrimSpace(storageDeps.ShowOption(socketPath, optAutosaveMax)); v != "" {
		return parseAutosaveMax(v)
	}
	return 5
}

func ResolveAutosaveIconSeconds(socketPath string) int {
	if v := strings.TrimSpace(os.Getenv(envAutosaveIconSeconds)); v != "" {
		return parseAutosaveIconSeconds(v)
	}
	if v := strings.TrimSpace(storageDeps.ShowOption(socketPath, optAutosaveIconSeconds)); v != "" {
		return parseAutosaveIconSeconds(v)
	}
	return 0
}

func ResolveAutosaveIcon(socketPath string) string {
	if v := os.Getenv(envAutosaveIcon); v != "" {
		return v
	}
	if v := storageDeps.ShowOption(socketPath, optAutosaveIcon); v != "" {
		return v
	}
	return defaultAutosaveStatusIcon
}

func parseAutosaveInterval(value string) int {
	interval, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || interval <= 0 {
		return 0
	}
	return interval
}

func parseAutosaveMax(value string) int {
	max, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 5
	}
	if max < 1 {
		return 1
	}
	return max
}

func parseAutosaveIconSeconds(value string) int {
	iconSeconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || iconSeconds <= 0 {
		return 0
	}
	return iconSeconds
}

func AutoSaveName(ts time.Time) string {
	return "auto-" + ts.Format("2006-01-02T15-04-05")
}

func PruneAutoSaves(dir string, max int) error {
	if max < 1 {
		max = 1
	}
	entries, err := ListSaves(dir)
	if err != nil {
		return err
	}

	autoEntries := make([]SaveEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind == SaveKindAuto {
			autoEntries = append(autoEntries, entry)
		}
	}
	if len(autoEntries) <= max {
		return nil
	}

	for _, entry := range autoEntries[max:] {
		if err := os.Remove(entry.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing autosave %q: %w", entry.Path, err)
		}
		archive := paneArchivePath(entry.Path)
		if err := os.Remove(archive); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing autosave archive %q: %w", archive, err)
		}
	}
	return nil
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
	if v := storageDeps.ShowOption(socketPath, "@tmux-popup-control-restore-pane-contents"); v != "" {
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

// SaveFileExists reports whether any snapshot with the given name prefix exists
// in dir. Since named saves include timestamps, this globs for name_*.json.
// Returns false for names that fail validation.
func SaveFileExists(dir, name string) bool {
	if name == "" {
		return false
	}
	if ValidateSaveName(name) != nil {
		return false
	}
	matches, _ := filepath.Glob(filepath.Join(dir, name+"_*.json"))
	return len(matches) > 0
}

// RelativeTime returns a concise human-readable relative timestamp like
// "just now", "5m ago", "2h ago", "yesterday", or "3 days ago".
func RelativeTime(t, now time.Time) string {
	d := max(now.Sub(t), time.Duration(0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		switch {
		case days == 1:
			return "yesterday"
		case days < 30:
			return fmt.Sprintf("%dd ago", days)
		case days < 365:
			months := days / 30
			if months == 1 {
				return "1 month ago"
			}
			return fmt.Sprintf("%d months ago", months)
		default:
			years := days / 365
			if years == 1 {
				return "1 year ago"
			}
			return fmt.Sprintf("%d years ago", years)
		}
	}
}

// DisplayName returns a display-friendly name for a save entry. Named saves
// return the name as-is. Unnamed saves (UUID filenames) return a truncated
// 8-character UUID prefix.
func (e SaveEntry) DisplayName() string {
	if e.Name != "" {
		return e.Name
	}
	// extract UUID from filename: UUID_TIMESTAMP.json
	base := filepath.Base(e.Path)
	base = strings.TrimSuffix(base, ".json")
	// UUID is 36 chars (8-4-4-4-12); take the first 8 as short ID
	if len(base) >= 8 {
		return base[:8]
	}
	return base
}
