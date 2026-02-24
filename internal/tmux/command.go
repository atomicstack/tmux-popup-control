package tmux

import "strings"

func baseArgs(socketPath string) []string {
	if strings.TrimSpace(socketPath) == "" {
		return []string{}
	}
	return []string{"-S", socketPath}
}

// ListCommands returns the output of "tmux list-commands" via control mode.
func ListCommands(socketPath string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}

	return client.Command("list-commands")
}

// ListKeys returns the output of "tmux list-keys" via control mode.
func ListKeys(socketPath string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}

	return client.Command("list-keys")
}
