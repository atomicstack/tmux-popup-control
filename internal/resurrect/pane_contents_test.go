package resurrect

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePaneArchiveUsesPrivatePermissions(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "private.panes.tar.gz")

	if err := WritePaneArchive(archivePath, map[string]string{"dev:0.0": "secret"}); err != nil {
		t.Fatalf("WritePaneArchive: %v", err)
	}

	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected archive mode 0600, got %03o", got)
	}
}

func TestExtractPaneArchiveUsesPrivatePermissions(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "private.panes.tar.gz")
	destDir := filepath.Join(dir, "extracted")

	if err := WritePaneArchive(archivePath, map[string]string{"dev:0.0": "secret"}); err != nil {
		t.Fatalf("WritePaneArchive: %v", err)
	}
	if err := ExtractPaneArchive(archivePath, destDir); err != nil {
		t.Fatalf("ExtractPaneArchive: %v", err)
	}

	info, err := os.Stat(filepath.Join(destDir, "dev:0.0"))
	if err != nil {
		t.Fatalf("stat extracted file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected extracted file mode 0600, got %03o", got)
	}
}

// TestPaneArchiveRoundTrip writes an archive with three panes, extracts it,
// and verifies each file matches the original content.
func TestPaneArchiveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "snap.panes.tar.gz")
	destDir := filepath.Join(dir, "extracted")

	contents := map[string]string{
		"dev:0.0":    "vim session content\nline two\n",
		"dev:0.1":    "bash prompt here\n$ ls -la\n",
		"shells:1.0": "another shell\nsome output\n",
	}

	if err := WritePaneArchive(archivePath, contents); err != nil {
		t.Fatalf("WritePaneArchive: %v", err)
	}

	if err := ExtractPaneArchive(archivePath, destDir); err != nil {
		t.Fatalf("ExtractPaneArchive: %v", err)
	}

	for name, want := range contents {
		p := filepath.Join(destDir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("reading extracted file %q: %v", name, err)
			continue
		}
		if got := string(data); got != want {
			t.Errorf("file %q: got %q, want %q", name, got, want)
		}
	}
}

// TestPaneArchiveEmpty writes an empty map and verifies the archive is created
// but contains no entries.
func TestPaneArchiveEmpty(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "empty.panes.tar.gz")
	destDir := filepath.Join(dir, "extracted")

	if err := WritePaneArchive(archivePath, map[string]string{}); err != nil {
		t.Fatalf("WritePaneArchive: %v", err)
	}

	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive not created: %v", err)
	}

	if err := ExtractPaneArchive(archivePath, destDir); err != nil {
		t.Fatalf("ExtractPaneArchive: %v", err)
	}

	entries, err := os.ReadDir(destDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("reading destDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty destDir, got %d entries", len(entries))
	}
}

// TestPaneArchiveExtractNonExistent verifies that extracting from a missing
// file returns an error.
func TestPaneArchiveExtractNonExistent(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "extracted")

	err := ExtractPaneArchive(filepath.Join(dir, "ghost.panes.tar.gz"), destDir)
	if err == nil {
		t.Fatal("expected error for non-existent archive, got nil")
	}
}

// TestPaneArchivePathTraversal crafts a raw archive with a path-traversal entry
// and verifies that ExtractPaneArchive rejects it.
func TestPaneArchivePathTraversal(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "evil.panes.tar.gz")
	destDir := filepath.Join(dir, "extracted")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir destDir: %v", err)
	}

	// manually craft an archive with a path-traversal entry name
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	evilNames := []string{
		"../evil",
		"../../escape",
		"/absolute/path",
	}
	for _, name := range evilNames {
		body := []byte("evil content")
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader %q: %v", name, err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatalf("Write %q: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file close: %v", err)
	}

	err = ExtractPaneArchive(archivePath, destDir)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected 'invalid' in error, got: %v", err)
	}

	// verify no files were written to destDir
	entries, err2 := os.ReadDir(destDir)
	if err2 != nil {
		t.Fatalf("reading destDir: %v", err2)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files extracted, got %d", len(entries))
	}
}
