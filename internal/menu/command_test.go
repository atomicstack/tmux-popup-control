package menu

import (
	"fmt"
	"testing"
)

func TestLoadCommandMenuParsesOutput(t *testing.T) {
	restore := listCommandsFn
	t.Cleanup(func() { listCommandsFn = restore })

	listCommandsFn = func(string) (string, error) {
		return "attach-session (attach) [-dErx] [-c working-directory]\nbind-key (bind) [-nr] [-T key-table]\n", nil
	}

	items, err := loadCommandMenu(Context{SocketPath: "/tmp/test.sock"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "attach-session" {
		t.Fatalf("expected ID %q, got %q", "attach-session", items[0].ID)
	}
	if items[1].ID != "bind-key" {
		t.Fatalf("expected ID %q, got %q", "bind-key", items[1].ID)
	}
}

func TestLoadCommandMenuError(t *testing.T) {
	restore := listCommandsFn
	t.Cleanup(func() { listCommandsFn = restore })

	listCommandsFn = func(string) (string, error) {
		return "", fmt.Errorf("connection refused")
	}

	items, err := loadCommandMenu(Context{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if items != nil {
		t.Fatalf("expected nil items, got %v", items)
	}
}

func TestLoadCommandMenuEmptyOutput(t *testing.T) {
	restore := listCommandsFn
	t.Cleanup(func() { listCommandsFn = restore })

	listCommandsFn = func(string) (string, error) {
		return "", nil
	}

	items, err := loadCommandMenu(Context{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestRunCommandReturnsActionResult(t *testing.T) {
	cmd := RunCommand("/tmp/nonexistent.sock", "display-message hello")
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	result, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	// We expect an error because the socket doesn't exist, but the key thing
	// is that RunCommand returns an ActionResult.
	if result.Err == nil && result.Info == "" {
		t.Fatal("expected either error or info in ActionResult")
	}
}

func TestRunCommandStripsQuotes(t *testing.T) {
	restore := runCommandOutputFn
	t.Cleanup(func() { runCommandOutputFn = restore })

	var captured []string
	runCommandOutputFn = func(_ string, args ...string) ([]byte, error) {
		captured = args
		return []byte(""), nil
	}

	cases := []struct {
		command string
		want    []string
	}{
		{
			command: "set-option -g status-left 'test'",
			want:    []string{"set-option", "-g", "status-left", "test"},
		},
		{
			command: `set-option -g status-left "test "`,
			want:    []string{"set-option", "-g", "status-left", "test "},
		},
		{
			command: `set-option -g status-left hello\ world`,
			want:    []string{"set-option", "-g", "status-left", "hello world"},
		},
		{
			command: "set-option -g status-left ' hello '",
			want:    []string{"set-option", "-g", "status-left", " hello "},
		},
		{
			command: `set-option -g status-left " hello "`,
			want:    []string{"set-option", "-g", "status-left", " hello "},
		},
		{
			command: "set-option -g mouse on",
			want:    []string{"set-option", "-g", "mouse", "on"},
		},
	}
	for _, tc := range cases {
		captured = nil
		cmd := RunCommand("/tmp/test.sock", tc.command)
		cmd()
		if len(captured) != len(tc.want) {
			t.Errorf("%s: got %d args %v, want %d %v", tc.command, len(captured), captured, len(tc.want), tc.want)
			continue
		}
		for i := range captured {
			if captured[i] != tc.want[i] {
				t.Errorf("%s: arg[%d] = %q, want %q", tc.command, i, captured[i], tc.want[i])
			}
		}
	}
}

func TestRunCommandReturnsOutputOnSuccess(t *testing.T) {
	restore := runCommandOutputFn
	t.Cleanup(func() { runCommandOutputFn = restore })

	runCommandOutputFn = func(string, ...string) ([]byte, error) {
		return []byte("bind-key -T root C-b send-prefix\nbind-key -T root C-o rotate-window\n"), nil
	}

	cmd := RunCommand("/tmp/test.sock", "list-keys")
	msg := cmd()
	result, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Output != "bind-key -T root C-b send-prefix\nbind-key -T root C-o rotate-window" {
		t.Fatalf("unexpected output %q", result.Output)
	}
}

func TestRunCommandShowOptionsEmptyOutputSynthesizesPlaceholder(t *testing.T) {
	restore := runCommandOutputFn
	t.Cleanup(func() { runCommandOutputFn = restore })

	runCommandOutputFn = func(string, ...string) ([]byte, error) {
		return []byte(""), nil
	}

	cases := []struct {
		command string
		want    string
	}{
		{"show-options -g mouse", "[option mouse has no value]"},
		{"show-window-options -g main-pane-width", "[option main-pane-width has no value]"},
		{"show-options -gq status-left", "[option status-left has no value]"},
		{"show -g mouse", "[option mouse has no value]"},
		{"show-hooks -g pane-focus-in", "[hook pane-focus-in has no value]"},
	}
	for _, tc := range cases {
		cmd := RunCommand("/tmp/test.sock", tc.command)
		msg := cmd()
		result, ok := msg.(ActionResult)
		if !ok {
			t.Fatalf("%s: expected ActionResult, got %T", tc.command, msg)
		}
		if result.Err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.command, result.Err)
		}
		if result.Output != tc.want {
			t.Errorf("%s: expected Output %q, got %q", tc.command, tc.want, result.Output)
		}
	}
}

func TestRunCommandShowOptionsEmptyScopeSynthesizesPlaceholder(t *testing.T) {
	restore := runCommandOutputFn
	t.Cleanup(func() { runCommandOutputFn = restore })

	runCommandOutputFn = func(string, ...string) ([]byte, error) {
		return []byte(""), nil
	}

	cases := []struct {
		command string
		want    string
	}{
		{"show-options -g", "[no options found in scope -g]"},
		{"show-options -s", "[no options found in scope -s]"},
		{"show-options -w", "[no options found in scope -w]"},
		{"show-options -p", "[no options found in scope -p]"},
		{"show-options -gq", "[no options found in scope -g]"},
		{"show-window-options -g", "[no options found in scope -g]"},
		{"show-hooks -g", "[no hooks found in scope -g]"},
		{"show-hooks", "[no hooks found]"},
	}
	for _, tc := range cases {
		cmd := RunCommand("/tmp/test.sock", tc.command)
		msg := cmd()
		result := msg.(ActionResult)
		if result.Output != tc.want {
			t.Errorf("%s: expected %q, got %q", tc.command, tc.want, result.Output)
		}
	}
}

func TestRunCommandShowOptionsNoScopeStaysEmpty(t *testing.T) {
	restore := runCommandOutputFn
	t.Cleanup(func() { runCommandOutputFn = restore })

	runCommandOutputFn = func(string, ...string) ([]byte, error) {
		return []byte(""), nil
	}

	// Without a scope flag the query targets the current context; an
	// unexpected empty result is not actionable information to surface.
	cmd := RunCommand("/tmp/test.sock", "show-options")
	msg := cmd()
	result := msg.(ActionResult)
	if result.Output != "" {
		t.Errorf("expected empty Output for bare show-options, got %q", result.Output)
	}
}

func TestRunCommandEmptyReturnsError(t *testing.T) {
	cmd := RunCommand("/tmp/test.sock", "")
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	result, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	if result.Err == nil {
		t.Fatal("expected error for empty command")
	}
}
