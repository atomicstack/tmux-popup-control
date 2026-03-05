package gotmuxcc

import (
	"fmt"
	"strings"
)

func (t *Tmux) SetOption(target, key, value, level string) error {
	q := t.query().
		cmd("set-option")

	if level != "" {
		q.fargs(level)
	}

	q.fargs("-t", target).
		pargs(key, value)

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to set option: %w", err)
	}

	return nil
}

func (t *Tmux) Option(target, key, level string) (*Option, error) {
	q := t.query().
		cmd("show-option")
	if level != "" {
		q.fargs(level)
	}
	q.fargs("-t", target).
		fargs("-v", key)

	output, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve option: %w", err)
	}

	raw := strings.TrimSpace(output.raw())
	return newOption(key, raw), nil
}

func (t *Tmux) Options(target, level string) ([]*Option, error) {
	q := t.query().
		cmd("show-options")
	if level != "" {
		q.fargs(level)
	}
	q.fargs("-t", target)

	output, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve options: %w", err)
	}

	return output.toOptions(), nil
}

func (t *Tmux) DeleteOption(target, key, level string) error {
	q := t.query().
		cmd("set-option")
	if level != "" {
		q.fargs(level)
	}
	q.fargs("-t", target).
		fargs("-u", key)

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to delete option: %w", err)
	}
	return nil
}

func (t *Tmux) Command(parts ...string) (string, error) {
	command, err := buildCommand(parts)
	if err != nil {
		return "", err
	}
	result, err := t.runCommand(command)
	if err != nil {
		return "", fmt.Errorf("failed to run command: %w", err)
	}
	return strings.Join(result.Lines, "\n"), nil
}

func newOption(key, value string) *Option {
	return &Option{Key: key, Value: value}
}

func (o *queryOutput) toOptions() []*Option {
	lines := o.result.Lines
	options := make([]*Option, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		options = append(options, newOption(parts[0], parts[1]))
	}
	return options
}

func buildCommand(parts []string) (string, error) {
	if len(parts) == 0 {
		return "", errEmptyCommand
	}
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, quoteArgument(part))
	}
	return strings.Join(escaped, " "), nil
}

func quoteArgument(arg string) string {
	if arg == "" {
		return "''"
	}
	if strings.ContainsAny(arg, " \t\n'\"\\#;{}~") {
		escaped := strings.ReplaceAll(arg, "'", "'\\''")
		return "'" + escaped + "'"
	}
	return arg
}
