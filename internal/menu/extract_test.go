package menu

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/extract"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestLoadExtractMenuWord(t *testing.T) {
	orig := extractCaptureFn
	extractCaptureFn = func(socket, target string) (string, error) {
		return "please make build", nil
	}
	defer func() { extractCaptureFn = orig }()

	items, err := loadExtractMenu(Context{ExtractCategory: extract.Word})
	if err != nil {
		t.Fatalf("loadExtractMenu: %v", err)
	}
	got := make([]string, len(items))
	for i, it := range items {
		got[i] = it.Label
	}
	// reverse order, min length 5.
	want := []string{"build", "please"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("labels = %v, want %v", got, want)
	}
	// item ID equals the token text (used verbatim by insert/copy).
	if items[0].ID != "build" {
		t.Fatalf("item[0].ID = %q, want %q", items[0].ID, "build")
	}
}

func TestCaptureForArea(t *testing.T) {
	t.Run("viewport", func(t *testing.T) {
		origCapture := extractCaptureFn
		origScrollback := extractScrollbackFn
		origWindowPanes := extractWindowPanesFn
		defer func() {
			extractCaptureFn = origCapture
			extractScrollbackFn = origScrollback
			extractWindowPanesFn = origWindowPanes
		}()

		var capturedTarget string
		extractCaptureFn = func(socket, target string) (string, error) {
			capturedTarget = target
			return "viewport text", nil
		}
		extractScrollbackFn = func(socket, target string) (string, error) {
			t.Fatalf("extractScrollbackFn should not be called for viewport")
			return "", nil
		}
		extractWindowPanesFn = func(socket, target string) ([]string, error) {
			t.Fatalf("extractWindowPanesFn should not be called for viewport")
			return nil, nil
		}

		got, err := captureForArea(Context{ExtractGrabArea: extract.Viewport})
		if err != nil {
			t.Fatalf("captureForArea: %v", err)
		}
		if got != "viewport text" {
			t.Fatalf("captureForArea() = %q, want %q", got, "viewport text")
		}
		if want := tmux.OriginPaneID(); capturedTarget != want {
			t.Fatalf("extractCaptureFn target = %q, want %q", capturedTarget, want)
		}
	})

	t.Run("pane-history", func(t *testing.T) {
		origCapture := extractCaptureFn
		origScrollback := extractScrollbackFn
		origWindowPanes := extractWindowPanesFn
		defer func() {
			extractCaptureFn = origCapture
			extractScrollbackFn = origScrollback
			extractWindowPanesFn = origWindowPanes
		}()

		extractCaptureFn = func(socket, target string) (string, error) {
			t.Fatalf("extractCaptureFn should not be called for pane-history")
			return "", nil
		}
		extractScrollbackFn = func(socket, target string) (string, error) {
			return "scrollback text", nil
		}
		extractWindowPanesFn = func(socket, target string) ([]string, error) {
			t.Fatalf("extractWindowPanesFn should not be called for pane-history")
			return nil, nil
		}

		got, err := captureForArea(Context{ExtractGrabArea: extract.PaneHistory})
		if err != nil {
			t.Fatalf("captureForArea: %v", err)
		}
		if got != "scrollback text" {
			t.Fatalf("captureForArea() = %q, want %q", got, "scrollback text")
		}
	})

	t.Run("window", func(t *testing.T) {
		origCapture := extractCaptureFn
		origScrollback := extractScrollbackFn
		origWindowPanes := extractWindowPanesFn
		defer func() {
			extractCaptureFn = origCapture
			extractScrollbackFn = origScrollback
			extractWindowPanesFn = origWindowPanes
		}()

		var capturedIDs []string
		extractWindowPanesFn = func(socket, target string) ([]string, error) {
			return []string{"%1", "%2"}, nil
		}
		extractCaptureFn = func(socket, target string) (string, error) {
			capturedIDs = append(capturedIDs, target)
			switch target {
			case "%1":
				return "capA", nil
			case "%2":
				return "capB", nil
			}
			return "", nil
		}
		extractScrollbackFn = func(socket, target string) (string, error) {
			t.Fatalf("extractScrollbackFn should not be called for window")
			return "", nil
		}

		got, err := captureForArea(Context{ExtractGrabArea: extract.Window})
		if err != nil {
			t.Fatalf("captureForArea: %v", err)
		}
		if got != "capA\ncapB" {
			t.Fatalf("captureForArea() = %q, want %q", got, "capA\ncapB")
		}
		if len(capturedIDs) != 2 || capturedIDs[0] != "%1" || capturedIDs[1] != "%2" {
			t.Fatalf("extractCaptureFn call order = %v, want [%%1 %%2]", capturedIDs)
		}
	})

	t.Run("window-history", func(t *testing.T) {
		origCapture := extractCaptureFn
		origScrollback := extractScrollbackFn
		origWindowPanes := extractWindowPanesFn
		defer func() {
			extractCaptureFn = origCapture
			extractScrollbackFn = origScrollback
			extractWindowPanesFn = origWindowPanes
		}()

		var capturedIDs []string
		extractWindowPanesFn = func(socket, target string) ([]string, error) {
			return []string{"%1", "%2"}, nil
		}
		extractScrollbackFn = func(socket, target string) (string, error) {
			capturedIDs = append(capturedIDs, target)
			switch target {
			case "%1":
				return "histA", nil
			case "%2":
				return "histB", nil
			}
			return "", nil
		}
		extractCaptureFn = func(socket, target string) (string, error) {
			t.Fatalf("extractCaptureFn should not be called for window-history")
			return "", nil
		}

		got, err := captureForArea(Context{ExtractGrabArea: extract.WindowHistory})
		if err != nil {
			t.Fatalf("captureForArea: %v", err)
		}
		if got != "histA\nhistB" {
			t.Fatalf("captureForArea() = %q, want %q", got, "histA\nhistB")
		}
		if len(capturedIDs) != 2 || capturedIDs[0] != "%1" || capturedIDs[1] != "%2" {
			t.Fatalf("extractScrollbackFn call order = %v, want [%%1 %%2]", capturedIDs)
		}
	})
}

func TestExtractRegisteredAsCategory(t *testing.T) {
	if _, ok := CategoryLoaders()["extract"]; !ok {
		t.Fatal("extract not in CategoryLoaders")
	}
	found := false
	for _, it := range RootItems() {
		if it.ID == "extract" {
			found = true
		}
	}
	if !found {
		t.Fatal("extract not in RootItems")
	}
}
