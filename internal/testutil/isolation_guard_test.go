package testutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// isolationExempt lists test packages that cannot import testutil (it imports
// them, so the import would cycle). Each must stay unable to start a process.
var isolationExempt = []string{"internal/shquote"}

// TestEveryTestPackageIsolatesFromUserServer fails when a test package's
// TestMain does not call IsolateFromUserServer. Without it, a test that reaches
// a socketless `tmux` exec acts on the developer's live server — which is how
// `make test` run inside tmux once killed real user sessions.
func TestEveryTestPackageIsolatesFromUserServer(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	testDirs := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".worktrees", ".gocache", ".gomodcache", ".git", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			testDirs[filepath.Dir(path)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(testDirs) < 10 {
		t.Fatalf("found only %d test packages under %s; the walk is broken", len(testDirs), root)
	}

	for dir := range testDirs {
		rel, _ := filepath.Rel(root, dir)
		rel = filepath.ToSlash(rel)
		files, _ := filepath.Glob(filepath.Join(dir, "*_test.go"))
		var hasMain, isolates bool
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			body := string(src)
			if strings.Contains(body, "func TestMain(") {
				hasMain = true
				isolates = isolates || strings.Contains(body, "IsolateFromUserServer()")
			}
		}
		if slices.Contains(isolationExempt, rel) {
			srcs, _ := filepath.Glob(filepath.Join(dir, "*.go"))
			for _, f := range srcs {
				src, _ := os.ReadFile(f)
				if strings.Contains(string(src), `"os/exec"`) {
					t.Errorf("%s is exempt from isolation but %s imports os/exec", rel, filepath.Base(f))
				}
			}
			continue
		}
		if !hasMain || !isolates {
			t.Errorf("%s: TestMain must call testutil.IsolateFromUserServer() (has TestMain=%v)", rel, hasMain)
		}
	}
}
