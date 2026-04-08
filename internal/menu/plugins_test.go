package menu

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

func TestMergeDeclaredPluginMetadataPreservesSourceForInstalledPlugins(t *testing.T) {
	installed := []plugin.Plugin{
		{Name: "maccyakto", Dir: "/tmp/plugins/maccyakto", Installed: true},
		{Name: "other", Dir: "/tmp/plugins/other", Installed: true},
	}
	declared := []plugin.Plugin{
		{Name: "maccyakto", Source: "github.com/atomicstack/maccyakto", Branch: "main"},
	}

	got := mergeDeclaredPluginMetadata(installed, declared)
	if got[0].Source != "github.com/atomicstack/maccyakto" {
		t.Fatalf("expected source to be copied onto installed plugin, got %q", got[0].Source)
	}
	if got[0].Branch != "main" {
		t.Fatalf("expected branch to be copied onto installed plugin, got %q", got[0].Branch)
	}
	if got[1].Source != "" {
		t.Fatalf("expected undeclared plugin source to remain empty, got %q", got[1].Source)
	}
}
