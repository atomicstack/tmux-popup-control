package tmux

// PopupStyle holds the optional styling applied to the floating pane the
// plugin opens as its popup. Empty fields are left to tmux's own defaults, so
// the plugin never imposes a look of its own.
type PopupStyle struct {
	BorderLines string // new-pane -B (see pane-border-lines)
	BorderStyle string // new-pane -S and -R
	Style       string // new-pane -s
}

// ResolvePopupStyle reads the popup styling from the
// @tmux-popup-control-popup-* options, the same ones the key-binding launcher
// in main.tmux expands.
func ResolvePopupStyle(socketPath string) PopupStyle {
	return PopupStyle{
		BorderLines: ShowOption(socketPath, "@tmux-popup-control-popup-border-lines"),
		BorderStyle: ShowOption(socketPath, "@tmux-popup-control-popup-border-style"),
		Style:       ShowOption(socketPath, "@tmux-popup-control-popup-style"),
	}
}

// Args returns the new-pane flags for the fields that are set. The popup is
// modal and therefore always the active pane, but the border style is applied
// to both the active (-S) and inactive (-R) borders so it holds either way.
func (p PopupStyle) Args() []string {
	var args []string
	if p.BorderLines != "" {
		args = append(args, "-B", p.BorderLines)
	}
	if p.BorderStyle != "" {
		args = append(args, "-S", p.BorderStyle, "-R", p.BorderStyle)
	}
	if p.Style != "" {
		args = append(args, "-s", p.Style)
	}
	return args
}
