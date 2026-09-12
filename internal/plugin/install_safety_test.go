package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestInstallPreservesExistingDestination(t *testing.T) {
	for _, kind := range []string{"directory", "empty directory", "file", "symlink", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "plugin")
			switch kind {
			case "directory", "empty directory":
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "directory" {
					if err := os.WriteFile(filepath.Join(dir, "local-work"), []byte("keep me"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "file":
				if err := os.WriteFile(dir, []byte("keep me"), 0600); err != nil {
					t.Fatal(err)
				}
			default:
				target := filepath.Join(base, "target")
				if kind == "symlink" {
					if err := os.Mkdir(target, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, dir); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat(dir)
			if err != nil {
				t.Fatal(err)
			}
			withStubGit(t, func(...string) ([]byte, error) { return nil, errors.New("clone failed") })
			if err := InstallOne(base, Plugin{Name: "plugin", Source: "user/plugin", Dir: dir}); err == nil {
				t.Fatal("expected existing destination error")
			}
			after, err := os.Lstat(dir)
			if err != nil {
				t.Fatalf("existing destination removed: %v", err)
			}
			if !os.SameFile(before, after) {
				t.Fatal("existing destination replaced")
			}
			if kind == "directory" {
				data, err := os.ReadFile(filepath.Join(dir, "local-work"))
				if err != nil || string(data) != "keep me" {
					t.Fatalf("local work changed: %q, %v", data, err)
				}
			}
		})
	}
}

func TestInstallCleansPartialClones(t *testing.T) {
	for _, fallbackSucceeds := range []bool{false, true} {
		t.Run(map[bool]string{false: "failure", true: "fallback"}[fallbackSucceeds], func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "plugin")
			calls := 0
			withStubGit(t, func(args ...string) ([]byte, error) {
				calls++
				dest := args[len(args)-1]
				if _, err := os.Stat(filepath.Join(dest, "partial")); err == nil {
					t.Error("fallback reused partial clone")
				}
				if err := os.MkdirAll(dest, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dest, "partial"), []byte("owned"), 0600); err != nil {
					t.Fatal(err)
				}
				if calls == 2 && fallbackSucceeds {
					return nil, nil
				}
				return nil, errors.New("clone failed")
			})
			err := InstallOne(base, Plugin{Name: "plugin", Source: "user/plugin", Dir: dir})
			if (err == nil) != fallbackSucceeds {
				t.Fatalf("unexpected result: %v", err)
			}
			if calls != 2 {
				t.Fatalf("got %d calls, want 2", calls)
			}
			entries, err := os.ReadDir(base)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if fallbackSucceeds {
				want = 1
			}
			if len(entries) != want {
				t.Fatalf("leftover clones: %v", entries)
			}
		})
	}
}

func TestInstallDoesNotReplaceDestinationCreatedDuringClone(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "plugin")
	withStubGit(t, func(args ...string) ([]byte, error) {
		if err := os.MkdirAll(args[len(args)-1], 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			t.Fatal(err)
		}
		return nil, nil
	})
	if err := InstallOne(base, Plugin{Name: "plugin", Source: "user/plugin", Dir: dir}); err == nil {
		t.Fatal("expected concurrent destination error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("concurrent directory overwritten")
	}
}

func TestConcurrentInstallPublishesOnlyOneClone(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "plugin")
	var ready sync.WaitGroup
	ready.Add(2)
	withStubGit(t, func(args ...string) ([]byte, error) {
		err := os.MkdirAll(args[len(args)-1], 0700)
		ready.Done()
		ready.Wait()
		return nil, err
	})
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- InstallOne(base, Plugin{Name: "plugin", Source: "user/plugin", Dir: dir}) }()
	}
	successes := 0
	for range 2 {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("got %d successful publications, want 1", successes)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "plugin" {
		t.Fatalf("unexpected install contents: %v", entries)
	}
}

func TestDuplicateInstallDeclarationsPreserveFirstClone(t *testing.T) {
	base := t.TempDir()
	p := Plugin{Name: "plugin", Source: "user/plugin", Dir: filepath.Join(base, "plugin")}
	calls := 0
	withStubGit(t, func(args ...string) ([]byte, error) {
		calls++
		dest := args[len(args)-1]
		if err := os.MkdirAll(dest, 0700); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(filepath.Join(dest, "local-work"), []byte("first clone"), 0600)
	})
	if err := Install(base, []Plugin{p, p}); err == nil {
		t.Fatal("expected duplicate destination error")
	}
	if calls != 1 {
		t.Fatalf("got %d clones, want 1", calls)
	}
	data, err := os.ReadFile(filepath.Join(p.Dir, "local-work"))
	if err != nil || string(data) != "first clone" {
		t.Fatalf("first clone changed: %q, %v", data, err)
	}
}

func TestInstalledDoesNotExposeCloneStaging(t *testing.T) {
	base := t.TempDir()
	p := Plugin{Name: "plugin", Source: "user/plugin", Dir: filepath.Join(base, "plugin")}
	withStubGit(t, func(args ...string) ([]byte, error) {
		installed, err := Installed(base)
		if err != nil {
			t.Fatal(err)
		}
		if len(installed) != 0 {
			t.Errorf("incomplete clone exposed as installed plugin: %+v", installed)
		}
		return nil, os.MkdirAll(args[len(args)-1], 0700)
	})
	if err := InstallOne(base, p); err != nil {
		t.Fatal(err)
	}
	installed, err := Installed(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 1 || installed[0].Name != "plugin" {
		t.Fatalf("published plugin missing: %+v", installed)
	}
}

func TestInstallOneClonesLocalRepository(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source.git")
	if out, err := defaultRunGitCommand("init", "--bare", source); err != nil {
		t.Fatalf("initializing local repository: %v: %s", err, out)
	}
	installDir := filepath.Join(base, "plugins")
	p := Plugin{Name: "plugin", Source: source, Dir: filepath.Join(installDir, "plugin")}
	if err := InstallOne(installDir, p); err != nil {
		t.Fatal(err)
	}
	if out, err := defaultRunGitCommand("-C", p.Dir, "rev-parse", "--is-inside-work-tree"); err != nil || string(out) != "true\n" {
		t.Fatalf("published clone is not usable: %v: %s", err, out)
	}
	entries, err := os.ReadDir(installDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != p.Name {
		t.Fatalf("unexpected installation contents: %v", entries)
	}
}
