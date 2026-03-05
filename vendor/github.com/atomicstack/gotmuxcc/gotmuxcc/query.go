package gotmuxcc

import (
	"errors"
	"fmt"
	"strings"
)

const querySeparator = "-:-"

type query struct {
	tmux      *Tmux
	command   []string
	flagArgs  []string
	posArgs   []string
	variables []string
}

func newQuery(t *Tmux) *query {
	return &query{
		tmux:     t,
		command:  make([]string, 0),
		flagArgs: make([]string, 0),
		posArgs:  make([]string, 0),
	}
}

func (q *query) cmd(parts ...string) *query {
	q.command = append(q.command, parts...)
	return q
}

func (q *query) fargs(args ...string) *query {
	q.flagArgs = append(q.flagArgs, args...)
	return q
}

func (q *query) pargs(args ...string) *query {
	q.posArgs = append(q.posArgs, args...)
	return q
}

func (q *query) vars(v ...string) *query {
	q.variables = append(q.variables[:0], v...)
	return q
}

func (q *query) build() (string, error) {
	if q.tmux == nil {
		return "", errors.New("gotmuxcc: query has no tmux instance")
	}
	if len(q.command) == 0 {
		return "", errors.New("gotmuxcc: query has no command")
	}

	parts := make([]string, 0, len(q.command)+len(q.flagArgs)+len(q.posArgs)+4)
	parts = append(parts, q.command...)
	for _, fa := range q.flagArgs {
		parts = append(parts, quoteArgument(fa))
	}

	if len(q.variables) > 0 {
		formats := make([]string, len(q.variables))
		for idx, variable := range q.variables {
			formats[idx] = fmt.Sprintf("#{%s}", variable)
		}
		format := strings.Join(formats, querySeparator)
		format = fmt.Sprintf("'%s'", format)
		if q.command[0] == "display-message" {
			parts = append(parts, "-p", format)
		} else {
			parts = append(parts, "-F", format)
		}
	}

	parts = append(parts, q.posArgs...)

	return strings.Join(parts, " "), nil
}

func (q *query) run() (*queryOutput, error) {
	command, err := q.build()
	if err != nil {
		return nil, err
	}

	result, err := q.tmux.runCommand(command)
	if err != nil {
		return nil, err
	}

	return &queryOutput{
		result:    result,
		variables: append([]string(nil), q.variables...),
	}, nil
}

type queryOutput struct {
	result    commandResult
	variables []string
}

func (o *queryOutput) collect() []queryResult {
	results := make([]queryResult, 0)
	if len(o.variables) == 0 {
		return results
	}

	for _, line := range o.result.Lines {
		if line == "" {
			continue
		}

		stripped := line
		if strings.HasPrefix(stripped, "'") {
			stripped = strings.TrimPrefix(stripped, "'")
			if strings.HasSuffix(stripped, "'") {
				stripped = strings.TrimSuffix(stripped, "'")
			}
		}

		values := strings.SplitN(stripped, querySeparator, len(o.variables))
		if len(values) < len(o.variables) {
			missing := len(o.variables) - len(values)
			extra := make([]string, missing)
			values = append(values, extra...)
		}

		entry := make(queryResult)
		for idx, variable := range o.variables {
			entry[variable] = values[idx]
		}
		results = append(results, entry)
	}

	return results
}

func (o *queryOutput) one() queryResult {
	collected := o.collect()
	if len(collected) == 0 {
		return queryResult{}
	}
	return collected[0]
}

func (o *queryOutput) raw() string {
	return strings.Join(o.result.Lines, "\n")
}

type queryResult map[string]string

func (q queryResult) get(key string) string {
	return q[key]
}
