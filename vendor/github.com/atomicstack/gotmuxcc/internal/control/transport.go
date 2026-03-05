package control

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/atomicstack/gotmuxcc/internal/trace"
)

// Config defines how a control-mode transport should be spawned.
type Config struct {
	TmuxBinary string
	SocketPath string
	ExtraArgs  []string
	Env        []string
}

// Transport manages a `tmux -C` subprocess and streams its output.
type Transport struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser

	lines chan string
	done  chan error

	sendMu    sync.Mutex
	closeOnce sync.Once
	closeErr  error
	finished  bool
	closing   bool
}

// ErrClosed indicates the transport is no longer available.
var ErrClosed = errors.New("control: transport closed")

// New launches tmux in control mode using the provided configuration.
func New(ctx context.Context, cfg Config) (*Transport, error) {
	if ctx == nil {
		return nil, errors.New("control: context must not be nil")
	}

	trace.Printf("transport", "New called binary=%q socket=%q extra=%v", cfg.TmuxBinary, cfg.SocketPath, cfg.ExtraArgs)

	bin := strings.TrimSpace(cfg.TmuxBinary)
	if bin == "" {
		bin = "tmux"
	}

	args := []string{"-C"}
	if cfg.SocketPath != "" {
		args = append(args, "-S", cfg.SocketPath)
	}
	args = append(args, cfg.ExtraArgs...)

	cmd := exec.CommandContext(ctx, bin, args...)
	if len(cfg.Env) > 0 {
		cmd.Env = append(cmd.Env[:0:0], cfg.Env...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("control: failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("control: failed to get stderr pipe: %w", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("control: failed to get stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("control: failed to start tmux: %w", err)
	}
	trace.Printf("transport", "tmux process started pid=%d args=%v", cmd.Process.Pid, cmd.Args)

	t := &Transport{
		cmd:   cmd,
		stdin: stdin,
		lines: make(chan string, 128),
		done:  make(chan error, 1),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Forward stdout lines.
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			text := scanner.Text()
			trace.Printf("transport", "recv <- %s", trace.FormatControlLine(text))
			select {
			case t.lines <- text:
			case <-ctx.Done():
				trace.Printf("transport", "stdout reader exiting due to context cancel")
				return
			}
		}
		if err := scanner.Err(); err != nil {
			t.finish(fmt.Errorf("control: stdout read error: %w", err))
			trace.Printf("transport", "stdout reader error=%v", err)
		}
	}()

	// Collect stderr so it can be surfaced if tmux exits unexpectedly.
	go func() {
		defer wg.Done()
		slurp, _ := io.ReadAll(stderr)
		if len(slurp) > 0 {
			payload := strings.TrimSpace(string(slurp))
			t.finish(errors.New(payload))
			trace.Printf("transport", "stderr=%s", trace.FormatControlLine(payload))
		}
	}()

	// Wait for process termination.
	go func() {
		wg.Wait()
		err := cmd.Wait()
		if err != nil && !errors.Is(err, context.Canceled) {
			t.finish(fmt.Errorf("control: tmux exit: %w", err))
			trace.Printf("transport", "tmux wait returned err=%v", err)
			return
		}
		trace.Printf("transport", "tmux wait completed err=%v", err)
		t.finish(nil)
	}()

	return t, nil
}

// Send writes a command line (with newline appended) to tmux.
func (t *Transport) Send(line string) error {
	if t == nil {
		return errors.New("control: transport is nil")
	}

	display := trace.FormatControlCommand(line)

	t.sendMu.Lock()
	defer t.sendMu.Unlock()

	if t.closeErr != nil {
		return t.closeErr
	}

	if t.finished {
		return ErrClosed
	}

	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}

	trace.Printf("transport", "send -> %s", display)

	if _, err := io.WriteString(t.stdin, line); err != nil {
		return fmt.Errorf("control: send failed: %w", err)
	}
	return nil
}

// Lines returns the channel streaming stdout lines from tmux.
func (t *Transport) Lines() <-chan string {
	if t == nil {
		return nil
	}
	return t.lines
}

// Done returns a channel that is closed when the transport terminates.
func (t *Transport) Done() <-chan error {
	if t == nil {
		return nil
	}
	return t.done
}

// Close terminates the tmux process and releases resources.
func (t *Transport) Close() error {
	if t == nil {
		return nil
	}

	trace.Printf("transport", "Close requested")

	t.closeOnce.Do(func() {
		t.sendMu.Lock()
		t.closing = true
		t.sendMu.Unlock()
		_ = t.stdin.Close()
		if err := t.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.finish(fmt.Errorf("control: failed to kill tmux: %w", err))
			trace.Printf("transport", "Close kill error=%v", err)
		}
	})

	return t.closeErr
}

func (t *Transport) finish(err error) {
	t.sendMu.Lock()
	defer t.sendMu.Unlock()

	if t.finished {
		return
	}
	t.finished = true
	if err != nil && !errors.Is(err, context.Canceled) {
		if t.closing {
			err = nil
		} else {
			t.closeErr = err
		}
	}
	close(t.lines)
	t.done <- t.closeErr
	close(t.done)
	trace.Printf("transport", "finish complete err=%v closing=%v finished=%v", t.closeErr, t.closing, t.finished)
}
