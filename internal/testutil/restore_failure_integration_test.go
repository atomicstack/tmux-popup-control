package testutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

// TestRestorePopupReportsFailureIntegration drives the real binary the way
// the restore-sessions subcommand does: a restore that cannot run (its pane
// archive is corrupt) must say so on screen and offer a way out, instead of
// freezing on the progress bar as every failed restore did until 2026-10-20.
func TestRestorePopupReportsFailureIntegration(t *testing.T) {
	RequireTmux(t)
	socket, cleanup, logDir := StartIsolatedTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })
	bin := BuildBinary(t)

	dir := t.TempDir()
	save := filepath.Join(dir, "broken_00000000-0000-0000-0000-000000000000_20261020T160000.json")
	sf := &resurrect.SaveFile{
		Version:         3,
		Timestamp:       time.Now(),
		Name:            "broken",
		HasPaneContents: true,
		Sessions: []resurrect.Session{{
			Name: "broken",
			Windows: []resurrect.Window{{
				Index: 0, Name: "shell", Layout: "tiled", Active: true,
				Panes: []resurrect.Pane{{Index: 0, Active: true}},
			}},
		}},
	}
	if err := resurrect.WriteSaveFile(save, sf); err != nil {
		t.Fatalf("write save file: %v", err)
	}
	archive := strings.TrimSuffix(save, ".json") + ".panes.tar.gz"
	if err := os.WriteFile(archive, []byte("this is not a tar.gz archive"), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	pane, _ := launchBinaryArgs(t, bin, socket, "popup", "", nil,
		"-socket", socket, "-width", "80", "-height", "24",
		"restore-sessions", "--resurrect-popup", "-socket", socket, "-from", save)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	WaitForContent(t, ctx, socket, pane, "restore failed")
	WaitForContent(t, ctx, socket, pane, "extracting pane archive")
	WaitForContent(t, ctx, socket, pane, "press any key")
}
