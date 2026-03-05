package gotmuxcc

import (
	"context"
	"errors"
)

// controlTransport is the low-level interface used by Tmux to communicate with
// a tmux process. It is designed to be satisfied by the control-mode transport.
type controlTransport interface {
	// Send writes a command line to the tmux control-mode connection.
	Send(cmd string) error
	// Lines streams raw lines received from tmux stdout in control mode.
	Lines() <-chan string
	// Done signals the transport has terminated and returns the exit error, if any.
	Done() <-chan error
	// Close terminates the transport and underlying tmux process.
	Close() error
}

// Dialer constructs a control transport for a given socket path.
type Dialer interface {
	Dial(ctx context.Context, socketPath string) (controlTransport, error)
}

// ConstructorOption customises how a Tmux instance is created.
type ConstructorOption func(*constructorConfig)

type constructorConfig struct {
	ctx    context.Context
	dialer Dialer
}

type DialerFunc func(ctx context.Context, socketPath string) (controlTransport, error)

func (f DialerFunc) Dial(ctx context.Context, socketPath string) (controlTransport, error) {
	return f(ctx, socketPath)
}

// WithContext allows overriding the context used to establish the transport.
func WithContext(ctx context.Context) ConstructorOption {
	return func(cfg *constructorConfig) {
		if ctx != nil {
			cfg.ctx = ctx
		}
	}
}

// WithDialer overrides the transport dialer, enabling dependency injection in tests.
func WithDialer(d Dialer) ConstructorOption {
	return func(cfg *constructorConfig) {
		if d != nil {
			cfg.dialer = d
		}
	}
}

// NewTmux initializes a Tmux client bound to the provided socket path.
// It mirrors the original gotmux constructor signature for compatibility.
func NewTmux(socketPath string) (*Tmux, error) {
	return NewTmuxWithOptions(socketPath)
}

// DefaultTmux initializes a Tmux client using tmux defaults (current socket).
func DefaultTmux() (*Tmux, error) {
	return NewTmux("")
}

// NewTmuxWithOptions creates a Tmux client with custom constructor options.
// It blocks until the initial control-mode handshake completes so that
// subsequent commands are not affected by the startup %begin/%end pair.
func NewTmuxWithOptions(socketPath string, opts ...ConstructorOption) (*Tmux, error) {
	cfg := constructorConfig{
		ctx:    context.Background(),
		dialer: defaultDialer{},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.dialer == nil {
		return nil, errors.New("gotmuxcc: no transport dialer configured")
	}
	transport, err := cfg.dialer.Dial(cfg.ctx, socketPath)
	if err != nil {
		return nil, err
	}
	t := &Tmux{
		transport: transport,
	}
	if socketPath != "" {
		socket, sockErr := newSocket(socketPath)
		if sockErr != nil {
			_ = transport.Close()
			return nil, sockErr
		}
		t.Socket = socket
	}
	t.router = newRouter(transport)

	// Wait for the initial %begin/%end from the attach/new-session to
	// be consumed so that the first user command isn't mismatched.
	select {
	case <-t.router.ready:
	case <-t.router.closed:
	case <-cfg.ctx.Done():
		_ = transport.Close()
		return nil, cfg.ctx.Err()
	}

	return t, nil
}

// Tmux is the entry point to the library.
type Tmux struct {
	Socket    *Socket
	transport controlTransport
	router    *router
}

// Close shuts down the underlying control-mode transport.
func (t *Tmux) Close() error {
	if t == nil {
		return nil
	}
	if t.router != nil {
		err := t.router.close()
		t.router = nil
		t.transport = nil
		t.Socket = nil
		return err
	}
	if t.transport != nil {
		err := t.transport.Close()
		t.transport = nil
		t.Socket = nil
		return err
	}
	return nil
}

// events exposes the router's asynchronous event stream.
func (t *Tmux) events() <-chan Event {
	if t == nil || t.router == nil {
		return nil
	}
	return t.router.eventsChannel()
}

func (t *Tmux) runCommand(command string) (commandResult, error) {
	if t == nil || t.router == nil {
		return commandResult{}, errRouterClosed
	}
	return t.router.runCommand(command)
}

func (t *Tmux) query() *query {
	return newQuery(t)
}

// defaultDialer implements Dialer using the control-mode transport.
type defaultDialer struct{}

func (d defaultDialer) Dial(ctx context.Context, socketPath string) (controlTransport, error) {
	return newControlTransport(ctx, socketPath)
}
