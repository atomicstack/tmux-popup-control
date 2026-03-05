package gotmuxcc

import (
	"strconv"
	"strings"
)

// Notification is implemented by all typed control-mode notifications.
// Use Event.Notification() to attempt parsing an Event into a typed form.
type Notification interface {
	notificationType() string
}

// OutputNotification represents a %output notification.
//
// Format: %output %<pane-id> <octal-encoded-data>
type OutputNotification struct {
	PaneID string // e.g. "%0"
	Data   []byte // decoded output bytes
}

func (OutputNotification) notificationType() string { return "output" }

// ExtendedOutputNotification represents a %extended-output notification
// sent when pause-after mode is active.
//
// Format: %extended-output %<pane-id> <age> : <octal-encoded-data>
type ExtendedOutputNotification struct {
	PaneID string // e.g. "%0"
	Age    uint64 // age in milliseconds
	Data   []byte // decoded output bytes
}

func (ExtendedOutputNotification) notificationType() string { return "extended-output" }

// LayoutChangeNotification represents a %layout-change notification.
//
// Format: %layout-change <window-id> <layout> <visible-layout> <flags>
type LayoutChangeNotification struct {
	WindowID      string // e.g. "@0"
	Layout        string // layout string
	VisibleLayout string // visible layout string
	Flags         string // raw flags string
}

func (LayoutChangeNotification) notificationType() string { return "layout-change" }

// SubscriptionChangedNotification represents a %subscription-changed notification.
//
// Formats:
//
//	%subscription-changed <name> $<sid> - - - : <value>
//	%subscription-changed <name> $<sid> @<wid> <idx> %<pid> : <value>
//	%subscription-changed <name> $<sid> @<wid> <idx> - : <value>
type SubscriptionChangedNotification struct {
	Name      string // subscription name
	SessionID string // e.g. "$0"
	WindowID  string // e.g. "@0" or "-" if not applicable
	Index     string // window index or "-"
	PaneID    string // e.g. "%0" or "-" if not applicable
	Value     string // the evaluated format string value
}

func (SubscriptionChangedNotification) notificationType() string { return "subscription-changed" }

// SessionChangedNotification represents a %session-changed notification.
//
// Format: %session-changed $<session-id> <name>
type SessionChangedNotification struct {
	SessionID string
	Name      string
}

func (SessionChangedNotification) notificationType() string { return "session-changed" }

// SessionRenamedNotification represents a %session-renamed notification.
//
// Format: %session-renamed $<session-id> <name>
type SessionRenamedNotification struct {
	SessionID string
	Name      string
}

func (SessionRenamedNotification) notificationType() string { return "session-renamed" }

// SessionWindowChangedNotification represents a %session-window-changed notification.
//
// Format: %session-window-changed $<session-id> @<window-id>
type SessionWindowChangedNotification struct {
	SessionID string
	WindowID  string
}

func (SessionWindowChangedNotification) notificationType() string { return "session-window-changed" }

// WindowAddNotification represents a %window-add notification.
//
// Format: %window-add @<window-id>
type WindowAddNotification struct {
	WindowID string
}

func (WindowAddNotification) notificationType() string { return "window-add" }

// WindowCloseNotification represents a %window-close notification.
//
// Format: %window-close @<window-id>
type WindowCloseNotification struct {
	WindowID string
}

func (WindowCloseNotification) notificationType() string { return "window-close" }

// WindowRenamedNotification represents a %window-renamed notification.
//
// Format: %window-renamed @<window-id> <name>
type WindowRenamedNotification struct {
	WindowID string
	Name     string
}

func (WindowRenamedNotification) notificationType() string { return "window-renamed" }

// WindowPaneChangedNotification represents a %window-pane-changed notification.
//
// Format: %window-pane-changed @<window-id> %<pane-id>
type WindowPaneChangedNotification struct {
	WindowID string
	PaneID   string
}

func (WindowPaneChangedNotification) notificationType() string { return "window-pane-changed" }

// UnlinkedWindowAddNotification represents a %unlinked-window-add notification.
//
// Format: %unlinked-window-add @<window-id>
type UnlinkedWindowAddNotification struct {
	WindowID string
}

func (UnlinkedWindowAddNotification) notificationType() string { return "unlinked-window-add" }

// UnlinkedWindowCloseNotification represents a %unlinked-window-close notification.
//
// Format: %unlinked-window-close @<window-id>
type UnlinkedWindowCloseNotification struct {
	WindowID string
}

func (UnlinkedWindowCloseNotification) notificationType() string { return "unlinked-window-close" }

// UnlinkedWindowRenamedNotification represents a %unlinked-window-renamed notification.
//
// Format: %unlinked-window-renamed @<window-id> <name>
type UnlinkedWindowRenamedNotification struct {
	WindowID string
	Name     string
}

func (UnlinkedWindowRenamedNotification) notificationType() string { return "unlinked-window-renamed" }

// PaneModeChangedNotification represents a %pane-mode-changed notification.
//
// Format: %pane-mode-changed %<pane-id>
type PaneModeChangedNotification struct {
	PaneID string
}

func (PaneModeChangedNotification) notificationType() string { return "pane-mode-changed" }

// ClientSessionChangedNotification represents a %client-session-changed notification.
//
// Format: %client-session-changed <client-name> $<session-id> <session-name>
type ClientSessionChangedNotification struct {
	ClientName  string
	SessionID   string
	SessionName string
}

func (ClientSessionChangedNotification) notificationType() string { return "client-session-changed" }

// ClientDetachedNotification represents a %client-detached notification.
//
// Format: %client-detached <client-name>
type ClientDetachedNotification struct {
	ClientName string
}

func (ClientDetachedNotification) notificationType() string { return "client-detached" }

// PasteBufferChangedNotification represents a %paste-buffer-changed notification.
//
// Format: %paste-buffer-changed <name>
type PasteBufferChangedNotification struct {
	Name string
}

func (PasteBufferChangedNotification) notificationType() string { return "paste-buffer-changed" }

// PasteBufferDeletedNotification represents a %paste-buffer-deleted notification.
//
// Format: %paste-buffer-deleted <name>
type PasteBufferDeletedNotification struct {
	Name string
}

func (PasteBufferDeletedNotification) notificationType() string { return "paste-buffer-deleted" }

// SessionsChangedNotification represents a %sessions-changed notification.
// This notification has no fields.
type SessionsChangedNotification struct{}

func (SessionsChangedNotification) notificationType() string { return "sessions-changed" }

// PauseNotification represents a %pause notification.
//
// Format: %pause %<pane-id>
type PauseNotification struct {
	PaneID string
}

func (PauseNotification) notificationType() string { return "pause" }

// ContinueNotification represents a %continue notification.
//
// Format: %continue %<pane-id>
type ContinueNotification struct {
	PaneID string
}

func (ContinueNotification) notificationType() string { return "continue" }

// ConfigErrorNotification represents a %config-error notification.
//
// Format: %config-error <text>
type ConfigErrorNotification struct {
	Message string
}

func (ConfigErrorNotification) notificationType() string { return "config-error" }

// MessageNotification represents a %message notification from display-message.
//
// Format: %message <text>
type MessageNotification struct {
	Message string
}

func (MessageNotification) notificationType() string { return "message" }

// ExitNotification represents a %exit notification.
//
// Format: %exit [<reason>]
type ExitNotification struct {
	Reason string
}

func (ExitNotification) notificationType() string { return "exit" }

// Notification attempts to parse the Event into a typed Notification.
// Returns nil if the event name is not recognized or the format is unexpected.
func (e Event) Notification() Notification {
	switch e.Name {
	case "output":
		return parseOutputNotification(e)
	case "extended-output":
		return parseExtendedOutputNotification(e)
	case "layout-change":
		return parseLayoutChangeNotification(e)
	case "subscription-changed":
		return parseSubscriptionChangedNotification(e)
	case "session-changed":
		return parseSessionIDNameNotification(e, func(id, name string) Notification {
			return &SessionChangedNotification{SessionID: id, Name: name}
		})
	case "session-renamed":
		return parseSessionIDNameNotification(e, func(id, name string) Notification {
			return &SessionRenamedNotification{SessionID: id, Name: name}
		})
	case "session-window-changed":
		if len(e.Fields) < 2 {
			return nil
		}
		return &SessionWindowChangedNotification{SessionID: e.Fields[0], WindowID: e.Fields[1]}
	case "window-add":
		if len(e.Fields) < 1 {
			return nil
		}
		return &WindowAddNotification{WindowID: e.Fields[0]}
	case "window-close":
		if len(e.Fields) < 1 {
			return nil
		}
		return &WindowCloseNotification{WindowID: e.Fields[0]}
	case "window-renamed":
		return parseWindowIDNameNotification(e, func(id, name string) Notification {
			return &WindowRenamedNotification{WindowID: id, Name: name}
		})
	case "window-pane-changed":
		if len(e.Fields) < 2 {
			return nil
		}
		return &WindowPaneChangedNotification{WindowID: e.Fields[0], PaneID: e.Fields[1]}
	case "unlinked-window-add":
		if len(e.Fields) < 1 {
			return nil
		}
		return &UnlinkedWindowAddNotification{WindowID: e.Fields[0]}
	case "unlinked-window-close":
		if len(e.Fields) < 1 {
			return nil
		}
		return &UnlinkedWindowCloseNotification{WindowID: e.Fields[0]}
	case "unlinked-window-renamed":
		return parseWindowIDNameNotification(e, func(id, name string) Notification {
			return &UnlinkedWindowRenamedNotification{WindowID: id, Name: name}
		})
	case "pane-mode-changed":
		if len(e.Fields) < 1 {
			return nil
		}
		return &PaneModeChangedNotification{PaneID: e.Fields[0]}
	case "client-session-changed":
		if len(e.Fields) < 3 {
			return nil
		}
		return &ClientSessionChangedNotification{
			ClientName:  e.Fields[0],
			SessionID:   e.Fields[1],
			SessionName: strings.Join(e.Fields[2:], " "),
		}
	case "client-detached":
		if len(e.Fields) < 1 {
			return nil
		}
		return &ClientDetachedNotification{ClientName: e.Fields[0]}
	case "sessions-changed":
		return &SessionsChangedNotification{}
	case "paste-buffer-changed":
		if len(e.Fields) < 1 {
			return nil
		}
		return &PasteBufferChangedNotification{Name: e.Fields[0]}
	case "paste-buffer-deleted":
		if len(e.Fields) < 1 {
			return nil
		}
		return &PasteBufferDeletedNotification{Name: e.Fields[0]}
	case "pause":
		if len(e.Fields) < 1 {
			return nil
		}
		return &PauseNotification{PaneID: e.Fields[0]}
	case "continue":
		if len(e.Fields) < 1 {
			return nil
		}
		return &ContinueNotification{PaneID: e.Fields[0]}
	case "config-error":
		return &ConfigErrorNotification{Message: e.Data}
	case "message":
		return &MessageNotification{Message: e.Data}
	case "exit":
		return &ExitNotification{Reason: e.Data}
	default:
		return nil
	}
}

// parseOutputNotification parses %output. The data portion after the pane ID
// is a single octal-encoded blob, not whitespace-separated fields.
func parseOutputNotification(e Event) Notification {
	// Raw format: "%output %<id> <octal-data>"
	// We need to parse from Raw because e.Data has already been field-split
	// and e.Fields would corrupt spaces in the data.
	paneID, payload, ok := splitOutputLine(e.Raw, "output")
	if !ok {
		return nil
	}
	return &OutputNotification{
		PaneID: paneID,
		Data:   decodeOctal(payload),
	}
}

// parseExtendedOutputNotification parses %extended-output.
// Format: %extended-output %<id> <age> : <octal-data>
func parseExtendedOutputNotification(e Event) Notification {
	paneID, rest, ok := splitOutputLine(e.Raw, "extended-output")
	if !ok {
		return nil
	}
	// rest is: "<age> : <data>"
	colonIdx := strings.Index(rest, " : ")
	if colonIdx < 0 {
		return nil
	}
	ageStr := strings.TrimSpace(rest[:colonIdx])
	age, err := strconv.ParseUint(ageStr, 10, 64)
	if err != nil {
		return nil
	}
	data := rest[colonIdx+3:]
	return &ExtendedOutputNotification{
		PaneID: paneID,
		Age:    age,
		Data:   decodeOctal(data),
	}
}

// parseLayoutChangeNotification parses %layout-change.
// Format: %layout-change @<wid> <layout> <visible-layout> <flags>
func parseLayoutChangeNotification(e Event) Notification {
	if len(e.Fields) < 4 {
		return nil
	}
	return &LayoutChangeNotification{
		WindowID:      e.Fields[0],
		Layout:        e.Fields[1],
		VisibleLayout: e.Fields[2],
		Flags:         strings.Join(e.Fields[3:], " "),
	}
}

// parseSubscriptionChangedNotification parses %subscription-changed.
// Format: %subscription-changed <name> $<sid> @<wid> <idx> %<pid> : <value>
//
//	or: %subscription-changed <name> $<sid> - - - : <value>
func parseSubscriptionChangedNotification(e Event) Notification {
	// Parse from Raw to avoid corruption of the colon-separated value.
	raw := e.Raw
	prefix := "%subscription-changed "
	if !strings.HasPrefix(raw, prefix) {
		return nil
	}
	rest := raw[len(prefix):]

	colonIdx := strings.Index(rest, " : ")
	if colonIdx < 0 {
		return nil
	}
	header := rest[:colonIdx]
	value := rest[colonIdx+3:]

	parts := strings.Fields(header)
	if len(parts) < 5 {
		return nil
	}
	return &SubscriptionChangedNotification{
		Name:      parts[0],
		SessionID: parts[1],
		WindowID:  parts[2],
		Index:     parts[3],
		PaneID:    parts[4],
		Value:     value,
	}
}

// splitOutputLine splits a raw notification line like "%output %0 data..."
// into the pane ID and the remaining data portion. The name parameter is
// the notification name without the leading %.
func splitOutputLine(raw, name string) (paneID, data string, ok bool) {
	prefix := "%" + name + " "
	if !strings.HasPrefix(raw, prefix) {
		return "", "", false
	}
	rest := raw[len(prefix):]

	// Next token is the pane ID (e.g. "%0").
	spaceIdx := strings.IndexByte(rest, ' ')
	if spaceIdx < 0 {
		// Just pane ID, no data.
		return rest, "", true
	}
	return rest[:spaceIdx], rest[spaceIdx+1:], true
}

func parseSessionIDNameNotification(e Event, build func(id, name string) Notification) Notification {
	if len(e.Fields) < 2 {
		return nil
	}
	// Name may contain spaces, so rejoin everything after the ID.
	name := strings.Join(e.Fields[1:], " ")
	return build(e.Fields[0], name)
}

func parseWindowIDNameNotification(e Event, build func(id, name string) Notification) Notification {
	if len(e.Fields) < 2 {
		return nil
	}
	name := strings.Join(e.Fields[1:], " ")
	return build(e.Fields[0], name)
}
