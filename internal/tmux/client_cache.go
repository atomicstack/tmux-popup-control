package tmux

import (
	"context"
	"sync"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

var (
	clientMu           sync.Mutex
	clientInitMu       sync.Mutex
	cachedClient       tmuxClient
	cachedSocket       string
	cachedCancel       context.CancelFunc
	initializingCancel context.CancelFunc
	clientGeneration   uint64

	// Stub transport creation in tests, keeping cache/lifecycle behavior real.
	dialTmux = func(ctx context.Context, socketPath string) (tmuxClient, error) {
		return gotmux.NewTmuxContext(ctx, socketPath)
	}
)

func sharedTmux(socketPath string) (tmuxClient, error) {
	clientMu.Lock()
	generation := clientGeneration
	if cachedClient != nil && cachedSocket == socketPath {
		client := cachedClient
		clientMu.Unlock()
		return client, nil
	}
	clientMu.Unlock()

	// Serialize initialization without preventing shutdown from cancelling it.
	clientInitMu.Lock()
	defer clientInitMu.Unlock()
	clientMu.Lock()
	if generation != clientGeneration {
		clientMu.Unlock()
		return nil, context.Canceled
	}
	if cachedClient != nil && cachedSocket == socketPath {
		client := cachedClient
		clientMu.Unlock()
		return client, nil
	}
	old, oldCancel := cachedClient, cachedCancel
	cachedClient, cachedSocket, cachedCancel = nil, "", nil
	ctx, cancel := context.WithCancel(context.Background())
	initializingCancel = cancel
	clientMu.Unlock()
	if oldCancel != nil {
		oldCancel()
	}
	if old != nil {
		_ = old.Close()
	}

	client, err := dialTmux(ctx, socketPath)
	if err == nil {
		configureControlClient(client)
	}
	clientMu.Lock()
	initializingCancel = nil
	if generation != clientGeneration || ctx.Err() != nil {
		err = context.Canceled
	}
	if err == nil {
		cachedClient = newTracedTmuxClient(socketPath, client)
		cachedSocket = socketPath
		cachedCancel = cancel
		client = cachedClient
	}
	clientMu.Unlock()
	if err != nil {
		cancel()
		if client != nil {
			_ = client.Close()
		}
		return nil, err
	}
	return client, nil
}

// Shutdown cancels connection initialization and closes the cached connection.
// No cache mutex is held while dialing, configuring, or closing a transport.
func Shutdown() {
	clientMu.Lock()
	clientGeneration++
	client, cancel, initCancel := cachedClient, cachedCancel, initializingCancel
	cachedClient, cachedSocket, cachedCancel = nil, "", nil
	clientMu.Unlock()
	if initCancel != nil {
		initCancel()
	}
	if cancel != nil {
		cancel()
	}
	if client != nil {
		_ = client.Close()
	}
	resetCaches()
}
