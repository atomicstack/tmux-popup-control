package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/atomicstack/tmux-popup-control/internal/app"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

var (
	socketFlag = flag.String("socket", "", "path to the tmux socket (overrides environment detection)")
	widthFlag  = flag.Int("width", 0, "desired viewport width in cells (0 uses terminal width)")
	heightFlag = flag.Int("height", 0, "desired viewport height in rows (0 uses terminal height)")
	footerFlag = flag.Bool("footer", false, "enable footer hint row (disabled by default)")
	traceFlag  = flag.Bool("trace", false, "enable verbose JSON trace logging")
)

func main() {
	flag.Parse()
	logging.SetTraceEnabled(*traceFlag)

	socketPath, err := tmux.ResolveSocketPath(*socketFlag)
	if err != nil {
		logging.Error(err)
		fmt.Fprintf(os.Stderr, "Error resolving tmux socket: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(socketPath, *widthFlag, *heightFlag, *footerFlag); err != nil {
		logging.Error(err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
