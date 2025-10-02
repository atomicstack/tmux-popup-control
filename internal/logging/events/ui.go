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
	logging.Trace("menu.enter", map[string]interface{}{
		"level":  levelID,
		"item":   itemID,
		"label":  label,
		"filter": filter,
	})
}

func (UITracer) MenuCursor(levelID string, cursor int) {
	logging.Trace("menu.cursor", map[string]interface{}{"level": levelID, "cursor": cursor})
}

func (ActionTracer) Error(err error) {
	if err == nil {
		return
	}
	logging.Trace("action.error", map[string]interface{}{"error": err.Error()})
}

func (ActionTracer) Success(info string) {
	logging.Trace("action.success", map[string]interface{}{"info": info})
}

func (FilterTracer) Cleared(levelID string) {
	logging.Trace("filter.clear", map[string]interface{}{"level": levelID})
}

func (FilterTracer) WordBackspace(levelID, filter string) {
	logging.Trace("filter.word-backspace", map[string]interface{}{"level": levelID, "filter": filter})
}

func (FilterTracer) Cursor(levelID string, pos int) {
	logging.Trace("filter.cursor", map[string]interface{}{"level": levelID, "cursor": pos})
}

func (FilterTracer) CursorWord(levelID string, pos int) {
	logging.Trace("filter.cursor-word", map[string]interface{}{"level": levelID, "cursor": pos})
}

func (FilterTracer) Append(levelID, filter string) {
	logging.Trace("filter.append", map[string]interface{}{"level": levelID, "filter": filter})
}

func (FilterTracer) Backspace(levelID, filter string) {
	logging.Trace("filter.backspace", map[string]interface{}{"level": levelID, "filter": filter})
}

func (CommandTracer) Queue(id, label string) {
	logging.Trace("command.queue", map[string]interface{}{"id": id, "label": label})
}

func (CommandTracer) Skip(id, label string) {
	logging.Trace("command.skip", map[string]interface{}{"id": id, "label": label})
}

func (CommandTracer) NoOp(id, label string) {
	logging.Trace("command.noop", map[string]interface{}{"id": id, "label": label})
}

func (CommandTracer) Result(id, label, msgType string) {
	logging.Trace("command.result", map[string]interface{}{"id": id, "label": label, "msg": msgType})
}
