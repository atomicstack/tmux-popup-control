package resurrect

import (
	"os"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

func TestMain(m *testing.M) {
	code := m.Run()
	testutil.ShutdownSharedServer()
	os.Exit(code)
}
