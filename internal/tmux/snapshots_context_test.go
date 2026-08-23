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
