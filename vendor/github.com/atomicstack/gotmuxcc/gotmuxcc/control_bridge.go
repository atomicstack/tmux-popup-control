package gotmuxcc

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/atomicstack/gotmuxcc/internal/control"
)

func newControlTransport(ctx context.Context, socketPath string) (controlTransport, error) {
	cfg := control.Config{
		SocketPath: socketPath,
		ExtraArgs:  initialAttachArgs("", socketPath),
	}
	transport, err := control.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("gotmux: failed to establish control transport: %w", err)
	}
	return transport, nil
}

func initialAttachArgs(tmuxBinary, socketPath string) []string {
	bin := strings.TrimSpace(tmuxBinary)
	if bin == "" {
		bin = "tmux"
	}
	target, err := discoverAttachTarget(bin, socketPath)
	if err != nil || target == "" {
		return nil
	}
	return []string{"attach-session", "-t", target}
}

func discoverAttachTarget(tmuxBinary, socketPath string) (string, error) {
	args := make([]string, 0, 6)
	if strings.TrimSpace(socketPath) != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "list-sessions", "-F", "#{session_name}")
	cmd := exec.Command(tmuxBinary, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			return name, nil
		}
	}
	return "", nil
}
