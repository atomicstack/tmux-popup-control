package tmux

import (
	"context"
	"errors"
	"strings"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// TestFetchContextCancelledBeforeWork verifies each ...Context fetcher checks
// ctx.Err() before doing any work — in particular before calling newTmux,
// which would otherwise start a fresh tmux round-trip for a watcher that has
// already given up.
func TestFetchContextCancelledBeforeWork(t *testing.T) {
	tests := []struct {
		name string
		call func(ctx context.Context, socketPath string) error
	}{
		{
			name: "sessions",
			call: func(ctx context.Context, socketPath string) error {
				_, err := FetchSessionsContext(ctx, socketPath)
				return err
			},
		},
		{
			name: "windows",
			call: func(ctx context.Context, socketPath string) error {
				_, err := FetchWindowsContext(ctx, socketPath)
				return err
			},
		},
		{
			name: "panes",
			call: func(ctx context.Context, socketPath string) error {
				_, err := FetchPanesContext(ctx, socketPath)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			withStubTmux(t, func(string) (tmuxClient, error) {
				called = true
				t.Fatalf("newTmux called with a cancelled context")
				return nil, nil
			})

			ctx, cancel := context.WithCancel(context.Background())
			cancel() // already cancelled

			err := tt.call(ctx, "sock")
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context.canceled, got %v", err)
			}
			if called {
				t.Fatalf("expected newTmux not to be called")
			}
		})
	}
}

// TestFetchSessionsContextPassesContextToExecFallback proves the context
// passed to FetchSessionsContext actually reaches the exec-based
// fetchSessionsFallback path (used when ListSessions returns no sessions),
// rather than being dropped in favour of context.Background().
func TestFetchSessionsContextPassesContextToExecFallback(t *testing.T) {
	fake := &fakeClient{
		sessions: nil, // empty triggers fetchSessionsFallback
		clients:  []*gotmux.Client{{Session: "dev"}},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	type sentinelKey struct{}
	ctx := context.WithValue(context.Background(), sentinelKey{}, "sentinel-value")

	var capturedCtx context.Context
	withStubCommanderContext(t, func(ctx context.Context, name string, args ...string) commander {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "list-sessions") {
			capturedCtx = ctx
			return stubCommander{output: []byte("dev\t1\t0\n")}
		}
		// show-options lookups (custom session format / switch-current) —
		// not under test here, return empty output so the rest of
		// FetchSessionsContext proceeds normally.
		return stubCommander{output: nil}
	})

	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	t.Setenv("TMUX_PANE", "")

	snap, err := FetchSessionsContext(ctx, "sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Sessions) != 1 {
		t.Fatalf("expected 1 session from exec fallback, got %d", len(snap.Sessions))
	}
	if capturedCtx == nil {
		t.Fatal("expected exec fallback to be invoked")
	}
	if got := capturedCtx.Value(sentinelKey{}); got != "sentinel-value" {
		t.Fatalf("expected ctx to carry sentinel value, got %v", got)
	}
}

// TestFetchSessionsWrapperStillWorks is a light regression check that the
// non-context FetchSessions wrapper still behaves like before now that its
// body has moved into FetchSessionsContext.
func TestFetchSessionsWrapperStillWorks(t *testing.T) {
	fake := &fakeClient{
		sessions: []*gotmux.Session{
			{Name: "dev", Windows: 2, Attached: 1, AttachedList: []string{"tty1"}},
		},
		clients: []*gotmux.Client{
			{Session: "dev"},
		},
		listSessionsFormatLines: []string{"dev\tcustom label"},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "#S: #{session_windows}w")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	t.Setenv("TMUX_PANE", "")

	snap, err := FetchSessions("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Current != "dev" {
		t.Fatalf("expected current dev, got %q", snap.Current)
	}
	if len(snap.Sessions) != 1 {
		t.Fatalf("expected single session, got %d", len(snap.Sessions))
	}
	if snap.Sessions[0].Label != "custom label" {
		t.Fatalf("expected custom label, got %q", snap.Sessions[0].Label)
	}
}

// contextCapturingClient wraps fakeClient to record the context.Context
// handed to each control-mode List*Context call, so tests can prove ctx
// flows all the way into the control-mode client itself — not just into the
// exec-based fallback/option-lookup helpers.
type contextCapturingClient struct {
	*fakeClient
	sessionsCtx context.Context
	windowsCtx  context.Context
	panesCtx    context.Context
}

func (c *contextCapturingClient) ListSessionsContext(ctx context.Context) ([]*gotmux.Session, error) {
	c.sessionsCtx = ctx
	return c.fakeClient.ListSessionsContext(ctx)
}

func (c *contextCapturingClient) ListAllWindowsContext(ctx context.Context) ([]*gotmux.Window, error) {
	c.windowsCtx = ctx
	return c.fakeClient.ListAllWindowsContext(ctx)
}

func (c *contextCapturingClient) ListAllPanesContext(ctx context.Context) ([]*gotmux.Pane, error) {
	c.panesCtx = ctx
	return c.fakeClient.ListAllPanesContext(ctx)
}

// TestFetchContextReachesControlModeListCalls proves the context passed to
// each Fetch*Context entrypoint now reaches the underlying control-mode
// List*Context call, not just the exec-based fallback/option-lookup paths.
// Before gotmuxcc v0.2.0 there was no per-command context API on the
// control-mode client, so this leg of the plumbing was uncancellable.
func TestFetchContextReachesControlModeListCalls(t *testing.T) {
	type sentinelKey struct{}
	sentinelCtx := context.WithValue(context.Background(), sentinelKey{}, "sentinel-value")

	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FILTER", "")
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_FILTER", "")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_FORMAT", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", "")
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_ID", "")
	t.Setenv("TMUX", "")

	tests := []struct {
		name   string
		call   func(ctx context.Context, socketPath string) error
		getCtx func(c *contextCapturingClient) context.Context
	}{
		{
			name: "sessions",
			call: func(ctx context.Context, socketPath string) error {
				_, err := FetchSessionsContext(ctx, socketPath)
				return err
			},
			getCtx: func(c *contextCapturingClient) context.Context { return c.sessionsCtx },
		},
		{
			name: "windows",
			call: func(ctx context.Context, socketPath string) error {
				_, err := FetchWindowsContext(ctx, socketPath)
				return err
			},
			getCtx: func(c *contextCapturingClient) context.Context { return c.windowsCtx },
		},
		{
			name: "panes",
			call: func(ctx context.Context, socketPath string) error {
				_, err := FetchPanesContext(ctx, socketPath)
				return err
			},
			getCtx: func(c *contextCapturingClient) context.Context { return c.panesCtx },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &contextCapturingClient{fakeClient: &fakeClient{
				sessions: []*gotmux.Session{{Name: "dev", Windows: 1}},
				clients:  []*gotmux.Client{{Session: "dev"}},
			}}
			withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
			withStubCommander(t, func(name string, args ...string) commander {
				return stubCommander{output: nil}
			})

			if err := tt.call(sentinelCtx, "sock"); err != nil {
				t.Fatalf("unexpected error for %s: %v", tt.name, err)
			}

			got := tt.getCtx(fake)
			if got == nil {
				t.Fatalf("expected %s list call to receive a context", tt.name)
			}
			if v := got.Value(sentinelKey{}); v != "sentinel-value" {
				t.Fatalf("expected ctx to carry sentinel value for %s, got %v", tt.name, v)
			}
		})
	}
}
