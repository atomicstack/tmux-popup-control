package tmux

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// withStubCommanderContext swaps runExecCommandContext so the cancellable
// exec-based WaitFor can be driven without a live tmux.
func withStubCommanderContext(t *testing.T, fn func(ctx context.Context, name string, args ...string) commander) {
	t.Helper()
	prev := runExecCommandContext
	runExecCommandContext = fn
	t.Cleanup(func() { runExecCommandContext = prev })
}

// blockingCommander models `tmux wait-for` on a channel that is never
// signaled: Run blocks until the context is cancelled (deadline or cancel),
// then surfaces ctx.Err() exactly as exec.CommandContext would after killing
// the process.
type blockingCommander struct{ ctx context.Context }

func (b blockingCommander) Run() error {
	<-b.ctx.Done()
	return b.ctx.Err()
}

func (b blockingCommander) Output() ([]byte, error) {
	<-b.ctx.Done()
	return nil, b.ctx.Err()
}

// TestWaitForSignaledReturnsNil verifies the happy path: when wait-for returns
// (channel signaled), WaitFor reports success and passes the right argv.
func TestWaitForSignaledReturnsNil(t *testing.T) {
	var gotArgs []string
	withStubCommanderContext(t, func(_ context.Context, name string, args ...string) commander {
		gotArgs = append([]string{name}, args...)
		return stubCommander{}
	})

	if err := WaitFor(context.Background(), "/tmp/socket", "chan-a", time.Second); err != nil {
		t.Fatalf("WaitFor returned error: %v", err)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "-S /tmp/socket") || !strings.Contains(joined, "wait-for chan-a") {
		t.Errorf("unexpected exec invocation: %q", joined)
	}
}

// TestWaitForTimesOut is the core regression test: a pane whose cat dies before
// signaling must not wedge restore forever. With a short timeout, WaitFor must
// return rather than block indefinitely.
func TestWaitForTimesOut(t *testing.T) {
	withStubCommanderContext(t, func(ctx context.Context, _ string, _ ...string) commander {
		return blockingCommander{ctx: ctx}
	})

	start := time.Now()
	err := WaitFor(context.Background(), "", "never-signaled", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("WaitFor blocked too long (%s); timeout not honored", elapsed)
	}
}

// TestWaitForRespectsCancelledContext verifies an externally cancelled context
// (e.g. the user aborting restore) unblocks WaitFor promptly even with no
// timeout set.
func TestWaitForRespectsCancelledContext(t *testing.T) {
	withStubCommanderContext(t, func(ctx context.Context, _ string, _ ...string) commander {
		return blockingCommander{ctx: ctx}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	if err := WaitFor(ctx, "", "chan", 0); err == nil {
		t.Fatal("expected error from cancelled context")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
