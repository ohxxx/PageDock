//go:build !windows

// [INPUT]: Window options and the saved application name.
// [OUTPUT]: Preserve macOS bundle naming, Linux window behavior, and window creation/display order.
// [POS]: Other-platform boundary for Windows-only taskbar properties.
// [PROTOCOL]: Update internal/appearance/AGENTS.md and platform validation scope.
package appearance

import (
	"github.com/egoist/mygo"
)

func PrepareTaskbar(string) error { return nil }

func NewWindow(options mygo.WindowOptions, _ string) (*mygo.Window, error) {
	return mygo.NewWindow(options), nil
}
