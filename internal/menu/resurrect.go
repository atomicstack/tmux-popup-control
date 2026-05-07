package menu

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

// ResurrectStart triggers a save or restore operation from a menu action.
type ResurrectStart struct {
	Operation string // "save" or "restore"
	Name      string // snapshot name (save-as only)
	SaveFile  string // path to restore from
	Config    resurrect.Config
}

// SaveAsPrompt requests interactive input for naming a snapshot.
type SaveAsPrompt struct {
	Context Context
	SaveDir string
}

// loadResurrectMenu lists the actions on the resurrect submenu.
func loadResurrectMenu(Context) ([]Item, error) {
	return []Item{
		{ID: "save", Label: "save"},
		{ID: "save-as", Label: "save-as"},
		{ID: "restore", Label: "restore"},
		{ID: "restore-from", Label: "restore-from"},
		{ID: "delete-saved", Label: "delete-saved"},
	}, nil
}

// ResurrectSaveAction triggers a manual save against the configured save dir.
func ResurrectSaveAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		dir, err := resurrect.ResolveDir(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: fmt.Errorf("resolving save dir: %w", err)}
		}
		return ResurrectStart{
			Operation: "save",
			Config: resurrect.Config{
				SocketPath:          ctx.SocketPath,
				SaveDir:             dir,
				CapturePaneContents: resurrect.ResolvePaneContents(ctx.SocketPath),
				ClientID:            ctx.ClientID,
			},
		}
	}
}

// ResurrectSaveAsAction prompts the user for a snapshot name before saving.
func ResurrectSaveAsAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		dir, err := resurrect.ResolveDir(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: fmt.Errorf("resolving save dir: %w", err)}
		}
		return SaveAsPrompt{Context: ctx, SaveDir: dir}
	}
}

// ResurrectRestoreAction restores from the most recent save.
func ResurrectRestoreAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		dir, err := resurrect.ResolveDir(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: fmt.Errorf("resolving save dir: %w", err)}
		}
		path, err := resurrect.LatestSave(dir)
		if err != nil {
			return ActionResult{Err: err}
		}
		return ResurrectStart{
			Operation: "restore",
			SaveFile:  path,
			Config: resurrect.Config{
				SocketPath:          ctx.SocketPath,
				SaveDir:             dir,
				CapturePaneContents: resurrect.ResolvePaneContents(ctx.SocketPath),
				ClientID:            ctx.ClientID,
			},
		}
	}
}

// ResurrectRestoreFromAction restores from a specific save chosen via the
// restore-from listing.
func ResurrectRestoreFromAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		dir, err := resurrect.ResolveDir(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: fmt.Errorf("resolving save dir: %w", err)}
		}
		return ResurrectStart{
			Operation: "restore",
			SaveFile:  item.ID,
			Config: resurrect.Config{
				SocketPath:          ctx.SocketPath,
				SaveDir:             dir,
				CapturePaneContents: resurrect.ResolvePaneContents(ctx.SocketPath),
				ClientID:            ctx.ClientID,
			},
		}
	}
}

// ResurrectDeleteSavedAction removes the save chosen from the delete-saved
// listing along with its companion pane-contents archive.
func ResurrectDeleteSavedAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	if target == "" {
		return func() tea.Msg {
			return ActionResult{Err: fmt.Errorf("invalid save target")}
		}
	}
	return func() tea.Msg {
		dir, err := resurrect.ResolveDir(ctx.SocketPath)
		if err != nil {
			// We can still try to delete by path even without the dir;
			// the symlink fix-up just won't run.
			dir = filepath.Dir(target)
		}
		if err := resurrect.DeleteSave(dir, target); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Deleted %s", filepath.Base(target))}
	}
}

func loadResurrectRestoreFromMenu(ctx Context) ([]Item, error) {
	return restoreListingItems(ctx)
}

func loadResurrectDeleteSavedMenu(ctx Context) ([]Item, error) {
	return restoreListingItems(ctx)
}

// restoreListingItems builds the same tabular listing of saves used by
// both restore-from and delete-saved.
func restoreListingItems(ctx Context) ([]Item, error) {
	dir, err := resurrect.ResolveDir(ctx.SocketPath)
	if err != nil {
		return nil, nil // empty list, no error shown
	}
	entries, err := resurrect.ListSaves(dir)
	if err != nil {
		return nil, nil
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// Reverse to oldest-first so the most recent entry is at the bottom.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	now := time.Now()
	alignments := []table.Alignment{
		table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignRight, table.AlignLeft,
	}
	headerRow := []string{"name", "type", "age", "date", "time", "size", "info"}
	rows := make([][]string, len(entries))
	ids := make([]string, len(entries))
	for i, e := range entries {
		saveType := string(e.Kind)
		if saveType == "" {
			saveType = string(resurrect.SaveKindManual)
		}
		name := e.DisplayName()
		if e.Kind == resurrect.SaveKindAuto {
			name = "auto"
		}
		age := resurrect.RelativeTime(e.Timestamp, now)
		date := e.Timestamp.Format("2006-01-02")
		timeStr := e.Timestamp.Format("15:04:05")
		size := humanizeSaveSize(e.Size)
		info := fmt.Sprintf("%2ds %3dw %3dp", e.SessionCount, e.WindowCount, e.PaneCount)
		if e.HasPaneContents {
			info += " +contents"
		}
		rows[i] = []string{name, saveType, age, date, timeStr, size, info}
		ids[i] = e.Path
	}
	aligned := formatRestoreRows(headerRow, rows, alignments)
	items := make([]Item, len(aligned))
	items[0] = Item{Label: aligned[0], Header: true}
	for i := 1; i < len(aligned); i++ {
		entry := entries[i-1]
		items[i] = Item{
			ID:          ids[i-1],
			Label:       aligned[i],
			StyledLabel: styleSaveEntryLine(aligned[i], entry.Kind),
		}
	}
	return items, nil
}

func formatRestoreRows(header []string, rows [][]string, alignments []table.Alignment) []string {
	allRows := append([][]string{header}, rows...)
	widths := restoreColumnWidths(allRows)
	out := make([]string, 1+len(rows))
	out[0] = formatRestoreRow(header, widths, nil)
	for i, row := range rows {
		out[i+1] = formatRestoreRow(row, widths, alignments)
	}
	return out
}

func restoreColumnWidths(rows [][]string) []int {
	if len(rows) == 0 {
		return nil
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			if w := len([]rune(cell)); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
}

func formatRestoreRow(row []string, widths []int, alignments []table.Alignment) string {
	var b strings.Builder
	for i, cell := range row {
		if i > 0 {
			b.WriteString("  ")
		}
		padding := widths[i] - len([]rune(cell))
		if padding < 0 {
			padding = 0
		}
		if i < len(alignments) && alignments[i] == table.AlignRight {
			b.WriteString(strings.Repeat(" ", padding))
			b.WriteString(cell)
			continue
		}
		b.WriteString(cell)
		b.WriteString(strings.Repeat(" ", padding))
	}
	return b.String()
}

func humanizeSaveSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, suffix := range units {
		value /= unit
		if value < unit || suffix == units[len(units)-1] {
			rounded := math.Round(value*10) / 10
			if rounded == math.Trunc(rounded) {
				return fmt.Sprintf("%.0f %s", rounded, suffix)
			}
			return fmt.Sprintf("%.1f %s", rounded, suffix)
		}
	}
	return fmt.Sprintf("%d B", size)
}

var (
	saveEntryFgManual = ansi.NewStyle().ForegroundColor(ansi.IndexedColor(33)).String()
	saveEntryFgAuto   = ansi.NewStyle().ForegroundColor(ansi.IndexedColor(93)).String()
	saveEntryFgReset  = ansi.NewStyle().ForegroundColor(nil).String()
)

func styleSaveEntryLine(line string, kind resurrect.SaveKind) string {
	switch kind {
	case resurrect.SaveKindAuto:
		return saveEntryFgAuto + line + saveEntryFgReset
	default:
		return saveEntryFgManual + line + saveEntryFgReset
	}
}
