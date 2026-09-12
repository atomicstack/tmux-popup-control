package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCancelledPluginOperationsDoNoWork(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "plugin")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := Plugin{Name: "plugin", Source: "user/plugin", Dir: dir, Installed: true}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	withStubGit(t, func(...string) ([]byte, error) { t.Error("git ran after cancellation"); return nil, nil })
	operations := map[string]func() error{
		"install":    func() error { return InstallOneContext(ctx, base, p) },
		"pull":       func() error { return UpdatePullOneContext(ctx, p) },
		"submodules": func() error { return UpdateSubmodulesOneContext(ctx, p) },
		"uninstall":  func() error { return UninstallContext(ctx, base, []Plugin{p}) },
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v, want cancellation", err)
			}
		})
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("cancelled uninstall removed plugin: %v", err)
	}
}

func TestInstallCancellationStopsFallbackAndPublication(t *testing.T) {
	for _, cloneSucceeds := range []bool{false, true} {
		t.Run(map[bool]string{false: "fallback", true: "publication"}[cloneSucceeds], func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "plugin")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			original := runGitCommandContext
			t.Cleanup(func() { runGitCommandContext = original })
			calls := 0
			runGitCommandContext = func(got context.Context, args ...string) ([]byte, error) {
				calls++
				if got != ctx {
					t.Error("git context not propagated")
				}
				if err := os.MkdirAll(args[len(args)-1], 0700); err != nil {
					t.Fatal(err)
				}
				cancel()
				if cloneSucceeds {
					return nil, nil
				}
				return nil, errors.New("interrupted clone")
			}
			err := InstallOneContext(ctx, base, Plugin{Name: "plugin", Source: "user/plugin", Dir: dir})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v, want cancellation", err)
			}
			if calls != 1 {
				t.Fatalf("got %d calls, want 1", calls)
			}
			entries, err := os.ReadDir(base)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("cancelled clone left files: %v", entries)
			}
		})
	}
}

func TestUpdateCancellationStopsSubmodules(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	original := runGitCommandContext
	t.Cleanup(func() { runGitCommandContext = original })
	calls := 0
	runGitCommandContext = func(got context.Context, args ...string) ([]byte, error) {
		calls++
		if got != ctx {
			t.Error("git context not propagated")
		}
		cancel()
		return nil, nil
	}
	err := UpdateOneContext(ctx, Plugin{Name: "plugin", Dir: t.TempDir(), Installed: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
	if calls != 1 {
		t.Fatalf("got %d git calls, want 1", calls)
	}
}

func TestDefaultGitHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := defaultRunGitCommandContext(ctx, "--version"); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
}

func TestGitCancellationStopsChildProcesses(t *testing.T) {
	base := t.TempDir()
	script := filepath.Join(base, "git")
	ready := filepath.Join(base, "ready")
	// The child inherits git's output pipes; cancelling only its parent hangs Wait.
	if err := os.WriteFile(script, []byte("#!/bin/sh\n/bin/sleep 5 &\necho $! > \"$1\"\nwait\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", base+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := defaultRunGitCommandContext(ctx, ready); result <- err }()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
waitReady:
	for {
		select {
		case err := <-result:
			t.Fatalf("git exited before ready: %v", err)
		case <-deadline:
			t.Fatal("git did not start")
		case <-ticker.C:
			if _, err := os.Stat(ready); err == nil {
				break waitReady
			}
		}
	}
	childData, err := os.ReadFile(ready)
	if err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(childData)))
	if err != nil || childPID <= 0 {
		t.Fatalf("invalid child pid %q: %v", childData, err)
	}
	child, err := os.FindProcess(childPID)
	if err != nil {
		t.Fatal(err)
	}
	// The shell records the exact child it spawned. Clean up only that owned pid.
	t.Cleanup(func() { _ = child.Kill(); _ = child.Release() })
	childRunning := func() bool {
		err := child.Signal(syscall.Signal(0))
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			return false
		}
		if err != nil {
			t.Fatalf("checking child process: %v", err)
		}
		if runtime.GOOS == "linux" {
			// Container init processes may leave a killed orphan as a zombie.
			data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(childPID), "stat"))
			if errors.Is(err, os.ErrNotExist) {
				return false
			}
			if err != nil {
				t.Fatal(err)
			}
			_, status, found := strings.CutLast(string(data), ")")
			if found && strings.HasPrefix(strings.TrimSpace(status), "Z") {
				return false
			}
		}
		return true
	}
	if !childRunning() {
		t.Fatal("git child exited before cancellation")
	}
	started := time.Now()
	cancel()
	err = <-result
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("cancelled git held child output pipes for %v", elapsed)
	}
	childDeadline := time.Now().Add(time.Second)
	for childRunning() {
		if time.Now().After(childDeadline) {
			t.Fatalf("git child %d is still running after cancellation", childPID)
		}
		<-ticker.C
	}
	// Release the handle now so deferred cleanup cannot target a reused pid.
	_ = child.Release()
}
