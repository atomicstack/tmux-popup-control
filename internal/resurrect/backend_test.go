package resurrect

import (
	"context"
	"testing"
	"time"
)

func TestBackendForUsesFileBackend(t *testing.T) {
	if got := BackendFor("sock").Name(); got != "file" {
		t.Fatalf("expected the file backend, got %q", got)
	}
}

func TestFileBackendSaveWritesSnapshot(t *testing.T) {
	dir := t.TempDir()
	stubSaveFetchers(t)
	t.Cleanup(withAutosaveNowFn(func() time.Time { return time.Date(2026, 10, 20, 1, 2, 3, 0, time.UTC) }))

	var last ProgressEvent
	for ev := range BackendFor("sock").Save(context.Background(), Config{SocketPath: "sock", SaveDir: dir, Name: "x"}) {
		last = ev
	}
	if !last.Done || last.Err != nil {
		t.Fatalf("save did not finish cleanly: %+v", last)
	}
	if entries, err := ListSaves(dir); err != nil || len(entries) != 1 {
		t.Fatalf("expected one snapshot, got %v (%v)", entries, err)
	}
}

func TestRunAutoSaveGoesThroughTheBackend(t *testing.T) {
	var saved Config
	orig := backendFor
	backendFor = func(string) Backend {
		return stubBackend{save: func(cfg Config) { saved = cfg }}
	}
	t.Cleanup(func() { backendFor = orig })

	if err := RunAutoSave(Config{SocketPath: "sock", SaveDir: t.TempDir()}, 1); err != nil {
		t.Fatal(err)
	}
	if saved.Kind != SaveKindAuto || saved.SocketPath != "sock" {
		t.Fatalf("autosave bypassed the backend: %+v", saved)
	}
}

type stubBackend struct{ save func(Config) }

func (stubBackend) Name() string { return "stub" }

func (b stubBackend) Save(_ context.Context, cfg Config) <-chan ProgressEvent {
	b.save(cfg)
	ch := make(chan ProgressEvent, 1)
	ch <- ProgressEvent{Done: true}
	close(ch)
	return ch
}

func (stubBackend) Restore(context.Context, Config, string) <-chan ProgressEvent {
	ch := make(chan ProgressEvent)
	close(ch)
	return ch
}
