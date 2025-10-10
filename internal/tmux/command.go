package tmux

import "strings"

func baseArgs(socketPath string) []string {
	if strings.TrimSpace(socketPath) == "" {
		return []string{}
	}
	return []string{"-S", socketPath}
}
