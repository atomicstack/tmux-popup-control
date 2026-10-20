package tmux

import (
	"reflect"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func TestMarkAutoSaveStartedRecordsUptime(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := MarkAutoSaveStarted("sock"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"set-option", "-gF", "@tmux-popup-control-autosave-last", "#{t/d:start_time}"}}
	if !reflect.DeepEqual(fake.commandCalls, want) {
		t.Fatalf("got %v, want %v", fake.commandCalls, want)
	}
}

func TestSetAutoSaveIndicatorSetsOptionAndRefreshesTerminalClients(t *testing.T) {
	fake := &fakeClient{clients: []*gotmux.Client{
		{Name: "/dev/ttys001", Session: "a"},
		{Name: "client-99", Session: "a", ControlMode: true},
		{Name: "/dev/ttys002", Session: "b"},
	}}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := SetAutoSaveIndicator("sock", "X"); err != nil {
		t.Fatal(err)
	}
	if err := SetAutoSaveIndicator("sock", ""); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"set-option", "-g", "@tmux-popup-control-autosave-saving", "X"},
		{"refresh-client", "-S", "-t", "/dev/ttys001"},
		{"refresh-client", "-S", "-t", "/dev/ttys002"},
		{"set-option", "-gu", "@tmux-popup-control-autosave-saving"},
		{"refresh-client", "-S", "-t", "/dev/ttys001"},
		{"refresh-client", "-S", "-t", "/dev/ttys002"},
	}
	if !reflect.DeepEqual(fake.commandCalls, want) {
		t.Fatalf("got %v\nwant %v", fake.commandCalls, want)
	}
}
