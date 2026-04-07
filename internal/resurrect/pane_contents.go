package resurrect

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// WritePaneArchive creates a .tar.gz archive at path containing one entry per
// pane. The key in contents is used as the tar entry filename (e.g. "dev:0.1")
// and the value is the plain-text pane content.
func WritePaneArchive(path string, contents map[string]string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("could not create pane archive %q: %w", path, err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, body := range contents {
		data := []byte(body)
		hdr := &tar.Header{
			Name: name,
			Mode: 0o600,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("could not write tar header for %q: %w", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			return fmt.Errorf("could not write tar entry for %q: %w", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("could not finalise tar archive: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("could not finalise gzip stream: %w", err)
	}
	return nil
}

// ExtractPaneArchive extracts the .tar.gz archive at archivePath into destDir.
// Each tar entry is written as a file named by the tar header. Entries whose
// names contain ".." or that are absolute paths are rejected to prevent path
// traversal; an error is returned on the first such entry and no files are
// written for that or subsequent entries.
func ExtractPaneArchive(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("could not open pane archive %q: %w", archivePath, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("could not read gzip stream from %q: %w", archivePath, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("could not read tar entry from %q: %w", archivePath, err)
		}

		if err := validateEntryName(hdr.Name); err != nil {
			return err
		}

		destPath := filepath.Join(destDir, hdr.Name)

		if err := os.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
			return fmt.Errorf("could not create directory for %q: %w", hdr.Name, err)
		}

		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return fmt.Errorf("could not create file %q: %w", destPath, err)
		}

		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return fmt.Errorf("could not write file %q: %w", destPath, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("could not close file %q: %w", destPath, closeErr)
		}
	}

	return nil
}

// validateEntryName rejects tar entry names that would escape destDir via path
// traversal (containing "..") or that are absolute paths.
func validateEntryName(name string) error {
	if filepath.IsAbs(name) {
		return fmt.Errorf("invalid tar entry name %q: absolute paths are not allowed", name)
	}
	if slices.Contains(strings.Split(filepath.ToSlash(name), "/"), "..") {
		return fmt.Errorf("invalid tar entry name %q: path traversal not allowed", name)
	}
	return nil
}
