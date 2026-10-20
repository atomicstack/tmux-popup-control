package tmux

import (
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// userOptionsOutput is what `show-options -g -F '#{option_name}=#{option_value}'`
// prints: raw, unquoted values, one option per line.
const userOptionsOutput = "@other=1\n" +
	"@tmux-popup-control-popup-style=fg=red,bg=blue\n" +
	"@tmux-popup-control-footer=on\n" +
	"@tmux-popup-control-session-format=a b\"c'd #{x}\n"

func TestPluginOptionsLoadsEveryPluginOptionInOneCommand(t *testing.T) {
	fake := &fakeClient{commandOutput: userOptionsOutput}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	withStubCommander(t, func(string, ...string) commander {
		t.Fatal("options must be read over control mode, not exec")
		return nil
	})

	got := PluginOptions("sock")
	want := map[string]string{
		"@tmux-popup-control-popup-style":    "fg=red,bg=blue",
		"@tmux-popup-control-footer":         "on",
		"@tmux-popup-control-session-format": "a b\"c'd #{x}",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}

	// every plugin option lookup is served from the one bulk read.
	for range 3 {
		_ = ShowOption("sock", "@tmux-popup-control-footer")
		_ = ShowOption("sock", "@tmux-popup-control-unset")
		_ = PluginOptions("sock")
	}
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected one show-options call, got %d: %v", len(fake.commandCalls), fake.commandCalls)
	}
	call := fake.commandCalls[0]
	if call[0] != "show-options" || !slices.Contains(call, "-g") || !slices.Contains(call, "-F") {
		t.Fatalf("unexpected command: %v", call)
	}
}

func TestPluginOptionsReloadsAfterInvalidation(t *testing.T) {
	var calls atomic.Int32
	fake := &fakeClient{commandFn: func(...string) (string, error) {
		if calls.Add(1) == 1 {
			return "@tmux-popup-control-footer=off", nil
		}
		return "@tmux-popup-control-footer=on", nil
	}}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if got := ShowOption("sock", "@tmux-popup-control-footer"); got != "off" {
		t.Fatalf("first read = %q", got)
	}
	InvalidateOptionCache()
	if got := ShowOption("sock", "@tmux-popup-control-footer"); got != "on" {
		t.Fatalf("read after invalidation = %q", got)
	}
}

func TestPluginOptionsErrorIsNotCached(t *testing.T) {
	fail := true
	fake := &fakeClient{commandFn: func(...string) (string, error) {
		if fail {
			return "", errors.New("boom")
		}
		return "@tmux-popup-control-footer=on", nil
	}}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if got := PluginOptions("sock"); len(got) != 0 {
		t.Fatalf("expected empty map on error, got %v", got)
	}
	fail = false
	if got := ShowOption("sock", "@tmux-popup-control-footer"); got != "on" {
		t.Fatalf("a failed load must not be cached, got %q", got)
	}
}

func TestOptionEnvOverridesTmuxOption(t *testing.T) {
	fake := &fakeClient{commandOutput: userOptionsOutput}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	t.Setenv("TMUX_POPUP_CONTROL_FOOTER", "")
	if got := Option("sock", "TMUX_POPUP_CONTROL_FOOTER", "@tmux-popup-control-footer"); got != "on" {
		t.Fatalf("empty env should fall through to the option, got %q", got)
	}
	t.Setenv("TMUX_POPUP_CONTROL_FOOTER", "off")
	if got := Option("sock", "TMUX_POPUP_CONTROL_FOOTER", "@tmux-popup-control-footer"); got != "off" {
		t.Fatalf("env should win, got %q", got)
	}
}

func TestShowOptionReadsBuiltinOptionsOverControlMode(t *testing.T) {
	var asked []string
	fake := &fakeClient{globalOptionFn: func(key string) (string, error) {
		asked = append(asked, key)
		return "  /bin/fish \n", nil
	}}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	withStubCommander(t, func(string, ...string) commander {
		t.Fatal("options must be read over control mode, not exec")
		return nil
	})

	for range 3 {
		if got := ShowOption("sock", "default-command"); got != "/bin/fish" {
			t.Fatalf("got %q", got)
		}
	}
	if strings.Join(asked, ",") != "default-command" {
		t.Fatalf("expected one cached lookup, got %v", asked)
	}
}

// withGlobalOptions stubs a control-mode client whose global options are opts.
func withGlobalOptions(t *testing.T, opts map[string]string) *fakeClient {
	t.Helper()
	fake := &fakeClient{globalOptionFn: func(key string) (string, error) {
		return opts[key], nil
	}}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	return fake
}
