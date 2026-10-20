package resurrect

import "context"

// Backend saves and restores whole-server snapshots, reporting progress as
// ProgressEvents on a channel that is closed after its Done event.
//
// The file backend is this package's own snapshot format, replayed with
// ordinary tmux commands. A native backend can slot in behind the same
// interface once tmux ships server-side save-state/restore-state; BackendFor
// is where it would be selected, e.g. when list-commands reports save-state.
type Backend interface {
	Name() string
	Save(ctx context.Context, cfg Config) <-chan ProgressEvent
	Restore(ctx context.Context, cfg Config, file string) <-chan ProgressEvent
}

type fileBackend struct{}

func (fileBackend) Name() string { return "file" }

func (fileBackend) Save(ctx context.Context, cfg Config) <-chan ProgressEvent {
	return Save(ctx, cfg)
}

func (fileBackend) Restore(ctx context.Context, cfg Config, file string) <-chan ProgressEvent {
	return Restore(ctx, cfg, file)
}

// backendFor is swapped in tests.
var backendFor = func(string) Backend { return fileBackend{} }

// BackendFor returns the snapshot backend to use for the server at
// socketPath.
func BackendFor(socketPath string) Backend {
	return backendFor(socketPath)
}
