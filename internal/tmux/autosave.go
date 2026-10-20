package tmux

// The autosave monitor hook in main.tmux compares the server's uptime with
// autoSaveLastOption; the status line shows autoSaveSavingOption.
const (
	autoSaveLastOption   = "@tmux-popup-control-autosave-last"
	autoSaveSavingOption = "@tmux-popup-control-autosave-saving"
)

// MarkAutoSaveStarted records the server uptime (#{t/d:start_time}) as the
// time of the last autosave, which turns the monitor's due format false until
// another interval has passed.
func MarkAutoSaveStarted(socketPath string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("set-option", "-gF", autoSaveLastOption, "#{t/d:start_time}")
	return err
}

// SetAutoSaveIndicator sets the status-line indicator to icon, or clears it
// when icon is empty, then redraws every terminal client's status line so the
// change shows straight away rather than at the next status-interval.
func SetAutoSaveIndicator(socketPath, icon string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	args := []string{"set-option", "-gu", autoSaveSavingOption}
	if icon != "" {
		args = []string{"set-option", "-g", autoSaveSavingOption, icon}
	}
	if _, err := client.Command(args...); err != nil {
		return err
	}
	clients, err := client.ListClients()
	if err != nil {
		return err
	}
	for _, c := range clients {
		if c == nil || c.ControlMode || c.Name == "" {
			continue
		}
		_, _ = client.Command("refresh-client", "-S", "-t", c.Name)
	}
	return nil
}
