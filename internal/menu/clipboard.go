package menu

import "os/exec"

func loadClipboardMenu(Context) ([]Item, error) {
	items := []Item{
		{ID: "buffer", Label: "Tmux Buffers"},
	}
	if _, err := exec.LookPath("copyq"); err == nil {
		items = append([]Item{{ID: "system", Label: "System Clipboard (copyq)"}}, items...)
	}
	return items, nil
}
