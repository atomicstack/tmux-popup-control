package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

type UITracer struct{}

type FilterTracer struct{}

type ActionTracer struct{}

type CommandTracer struct{}

var (
	UI      = UITracer{}
	Filter  = FilterTracer{}
	Action  = ActionTracer{}
	Command = CommandTracer{}
)

func (UITracer) MenuEnter(levelID, itemID, label, filter string) {
	logging.Trace("menu.enter", map[string]any{
		"level":  levelID,
		"item":   itemID,
		"label":  label,
		"filter": filter,
	})
}

func (UITracer) MenuCursor(levelID string, cursor int) {
	logging.Trace("menu.cursor", map[string]any{"level": levelID, "cursor": cursor})
}

func (ActionTracer) Error(err error) {
	if err == nil {
		return
	}
	logging.Trace("action.error", map[string]any{"error": err.Error()})
}

func (ActionTracer) Success(info string) {
	logging.Trace("action.success", map[string]any{"info": info})
}

func (FilterTracer) Cleared(levelID string) {
	logging.Trace("filter.clear", map[string]any{"level": levelID})
}

func (FilterTracer) WordBackspace(levelID, filter string) {
	logging.Trace("filter.word-backspace", map[string]any{"level": levelID, "filter": filter})
}

func (FilterTracer) Cursor(levelID string, pos int) {
	logging.Trace("filter.cursor", map[string]any{"level": levelID, "cursor": pos})
}

func (FilterTracer) CursorWord(levelID string, pos int) {
	logging.Trace("filter.cursor-word", map[string]any{"level": levelID, "cursor": pos})
}

func (FilterTracer) Append(levelID, filter string) {
	logging.Trace("filter.append", map[string]any{"level": levelID, "filter": filter})
}

func (FilterTracer) Backspace(levelID, filter string) {
	logging.Trace("filter.backspace", map[string]any{"level": levelID, "filter": filter})
}

func (CommandTracer) Queue(id, label string) {
	logging.Trace("command.queue", map[string]any{"id": id, "label": label})
}

func (CommandTracer) Skip(id, label string) {
	logging.Trace("command.skip", map[string]any{"id": id, "label": label})
}

func (CommandTracer) NoOp(id, label string) {
	logging.Trace("command.noop", map[string]any{"id": id, "label": label})
}

func (CommandTracer) Result(id, label, msgType string) {
	logging.Trace("command.result", map[string]any{"id": id, "label": label, "msg": msgType})
}
