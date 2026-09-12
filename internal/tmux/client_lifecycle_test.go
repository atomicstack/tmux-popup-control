package tmux

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Use the production connection factory, stubbing only transport creation.
func TestConnectionFactoryCachesRealInitialization(t *testing.T) {
	Shutdown()
	oldDial := dialTmux
	t.Cleanup(func() { Shutdown(); dialTmux = oldDial })
	calls := 0
	client := &fakeClient{}
	dialTmux = func(context.Context, string) (tmuxClient, error) { calls++; return client, nil }
	first, err := newTmux("one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := newTmux("one")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || calls != 1 {
		t.Fatalf("connection was not reused: dial count %d", calls)
	}
}

func TestShutdownCancelsConnectionInitialization(t *testing.T) {
	for _, phase := range []string{"dial", "configure"} {
		t.Run(phase, func(t *testing.T) {
			Shutdown()
			oldDial, oldConfigure := dialTmux, configureControlClient
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			t.Cleanup(func() { unblock(); Shutdown(); dialTmux = oldDial; configureControlClient = oldConfigure })
			var life context.Context
			dialTmux = func(ctx context.Context, _ string) (tmuxClient, error) {
				life = ctx
				if phase == "dial" {
					close(entered)
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-release:
					}
				}
				return &fakeClient{}, nil
			}
			configureControlClient = func(tmuxClient) {
				if phase == "configure" {
					close(entered)
					select {
					case <-life.Done():
					case <-release:
					}
				}
			}
			result := make(chan error, 1)
			go func() { _, err := newTmux("pending"); result <- err }()
			<-entered
			stopped := make(chan struct{})
			go func() { Shutdown(); close(stopped) }()
			select {
			case <-stopped:
			case <-time.After(time.Second):
				unblock()
				<-stopped
				<-result
				t.Fatal("shutdown waited for connection initialization instead of cancelling it")
			}
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("initialization returned %v after shutdown", err)
				}
			case <-time.After(time.Second):
				unblock()
				<-result
				t.Fatal("initialization did not observe cancellation")
			}
			if cachedClient != nil {
				t.Fatal("cancelled initialization published a cached client")
			}
		})
	}
}
